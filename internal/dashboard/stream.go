package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/agentforge/agentforge/internal/cost"
	"github.com/agentforge/agentforge/internal/llm"
	"github.com/agentforge/agentforge/internal/sse"
)

type streamChatRequest struct {
	Prompt      string  `json:"prompt"`
	Agent       string  `json:"agent"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req streamChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, "missing prompt", http.StatusBadRequest)
		return
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	client, err := s.sseHub.Connect(w)
	if err != nil {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	s.sseHub.Subscribe(client, "chat."+client.ID)

	go func() {
		s.streamChat(r.Context(), client, req, s.costTracker)
	}()

	<-client.Quit()
}

func (s *Server) streamChat(ctx context.Context, client *sse.Client, req streamChatRequest, tracker *cost.Tracker) {
	s.adapterMu.RLock()
	adapter := s.adapter
	s.adapterMu.RUnlock()

	if adapter == nil {
		s.sseHub.SendToClient(client, sse.NewErrorEvent("No LLM provider configured. Go to Settings → LLM and set a provider."))
		return
	}

	// Build message history from session (gives LLM real conversation context)
	agentID := req.Agent
	if agentID == "" {
		agentID = "agentforge"
	}
	history := s.sessionMgr.History(agentID)
	messages := make([]llm.Message, 0, len(history)+1)
	for _, t := range history {
		messages = append(messages, llm.Message{Role: t.Role, Content: t.Content})
	}
	messages = append(messages, llm.Message{Role: "user", Content: req.Prompt})

	// Persist user turn before streaming
	s.sessionMgr.AddTurn(agentID, "user", req.Prompt, 0) //nolint:errcheck

	s.sseHub.SendToClient(client, sse.NewStatusEvent(req.Agent, "streaming",
		"Processing with "+adapter.Provider()))

	llmReq := llm.Request{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: req.Temperature,
	}

	ch, err := adapter.StreamChat(ctx, llmReq)
	if err != nil {
		if errors.Is(err, llm.ErrStreamingNotSupported) {
			s.nonStreamingChat(ctx, client, llmReq, tracker)
			return
		}
		s.sseHub.SendToClient(client, sse.NewErrorEvent(fmt.Sprintf("Stream error: %v", err)))
		return
	}

	content, _, _, usage, drainErr := s.drainStream(ctx, client, ch, req.Model, tracker)
	if drainErr != nil {
		s.sseHub.SendToClient(client, sse.NewErrorEvent(drainErr.Error()))
		return
	}

	// Persist assistant response
	if content != "" {
		s.sessionMgr.AddTurn(agentID, "assistant", content, usage.CompletionTokens) //nolint:errcheck
	}

	u := &sse.Usage{}
	if usage.TotalTokens > 0 {
		u = &sse.Usage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
	}
	s.sseHub.SendToClient(client, sse.NewDoneEvent(req.Model, u))
}

func (s *Server) drainStream(ctx context.Context, client *sse.Client, ch <-chan llm.StreamChunk, model string, tracker *cost.Tracker) (string, []llm.ToolCall, string, llm.Usage, error) {
	var content strings.Builder
	var toolCalls []llm.ToolCall
	var finish string
	var usage llm.Usage
	var prevTotalCost float64

	for {
		select {
		case <-ctx.Done():
			return content.String(), toolCalls, finish, usage, ctx.Err()
		case chunk, ok := <-ch:
			if !ok {
				return content.String(), toolCalls, finish, usage, nil
			}
			if chunk.Error != "" {
				return content.String(), toolCalls, finish, usage, fmt.Errorf("llm stream: %s", chunk.Error)
			}
			if chunk.Content != "" {
				content.WriteString(chunk.Content)
				s.sseHub.SendToClient(client, sse.NewChunkEvent(chunk.Content))
			}
			if len(chunk.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.ToolCalls...)
				for _, tc := range chunk.ToolCalls {
					s.sseHub.SendToClient(client, sse.NewToolCallEvent(tc.ID, tc.Name, string(tc.Arguments), false, ""))
				}
			}
			if chunk.Finish != "" {
				finish = chunk.Finish
			}
			if chunk.Usage.TotalTokens > 0 {
				usage = chunk.Usage
				// Record cost when we have usage data and emit real-time cost update
				if tracker != nil {
					tracker.RecordWithCached(model, usage, 0, client.ID)
					currentTotalCost := tracker.GetTotalCost()
					deltaCost := currentTotalCost - prevTotalCost
					if deltaCost > 0 {
						s.sseHub.SendToClient(client, sse.NewCostUpdateEvent(
							client.ID,
							model,
							deltaCost,
							currentTotalCost,
							usage.PromptTokens,
							usage.CompletionTokens,
							0,
						))
						prevTotalCost = currentTotalCost
					}
				}
			}
			if chunk.Done {
				return content.String(), toolCalls, finish, usage, nil
			}
		}
	}
}

func (s *Server) nonStreamingChat(ctx context.Context, client *sse.Client, req llm.Request, tracker *cost.Tracker) {
	s.sseHub.SendToClient(client, sse.NewStatusEvent("", "streaming", "Fallback to batch mode"))

	s.adapterMu.RLock()
	adapter := s.adapter
	s.adapterMu.RUnlock()

	resp, err := adapter.Chat(ctx, req)
	if err != nil {
		s.sseHub.SendToClient(client, sse.NewErrorEvent(fmt.Sprintf("LLM error: %v", err)))
		return
	}
	if len(resp.Choices) == 0 {
		s.sseHub.SendToClient(client, sse.NewErrorEvent("Empty response from LLM"))
		return
	}

	s.sseHub.SendToClient(client, sse.NewChunkEvent(resp.Choices[0].Message.Content))

	// Record cost for non-streaming response
	if tracker != nil && resp.Usage.TotalTokens > 0 {
		tracker.RecordWithCached(resp.Model, resp.Usage, 0, client.ID)
	}

	u := &sse.Usage{}
	if resp.Usage.TotalTokens > 0 {
		u = &sse.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	s.sseHub.SendToClient(client, sse.NewDoneEvent(resp.Model, u))
}