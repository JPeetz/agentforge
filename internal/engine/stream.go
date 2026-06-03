package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/agentforge/internal/llm"
	"github.com/agentforge/agentforge/internal/memory"
)

// StreamEvent is emitted for each step in a streaming agent run.
type StreamEvent struct {
	Kind             string `json:"kind"`
	Content          string `json:"content,omitempty"`
	Tool             string `json:"tool,omitempty"`
	Args             string `json:"args,omitempty"`
	Error            string `json:"error,omitempty"`
	Model            string `json:"model,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}

// StreamCallback receives streaming events from RunStream.
type StreamCallback func(StreamEvent)

// RunStream executes a prompt with token-by-token streaming via the agent's LLM adapter.
func (a *Agent) RunStream(ctx context.Context, prompt, model string, temp float64, cb StreamCallback) (string, llm.Usage, error) {
	if a.adapter == nil {
		return "", llm.Usage{}, fmt.Errorf("no LLM adapter configured")
	}
	if model == "" {
		model = a.Model
	}

	cb(StreamEvent{Kind: "status", Content: fmt.Sprintf("Running with %s", a.adapter.Provider()), Model: model})

	messages := []llm.Message{{Role: "user", Content: prompt}}

	var tools []llm.ToolDef
	if a.registry != nil {
		for _, name := range a.registry.List() {
			t := a.registry.Get(name)
			meta := t.Meta()
			tools = append(tools, llm.ToolDef{
				Name:        meta.Name,
				Description: meta.Description,
				Parameters:  meta.InputSchema,
			})
		}
	}

	maxIter := a.cfg.MaxLoopIterations
	if maxIter == 0 {
		maxIter = 5
	}

	streamSupported := true
	var usage llm.Usage
	var full strings.Builder
	var functionResults []map[string]any
	toolCount := 0

	for iter := 0; iter < maxIter; iter++ {
		select {
		case <-ctx.Done():
			return full.String(), usage, ctx.Err()
		default:
		}

		req := llm.Request{
			Model:       model,
			Messages:    messages,
			MaxTokens:   4096,
			Temperature: temp,
			Tools:       tools,
		}

		if streamSupported {
			ch, err := a.adapter.StreamChat(ctx, req)
			if err != nil {
				if errors.Is(err, llm.ErrStreamingNotSupported) {
					streamSupported = false
					cb(StreamEvent{Kind: "status", Content: "Streaming not supported, falling back to batch mode"})
				} else {
					return full.String(), usage, fmt.Errorf("stream error: %w", err)
				}
			}

			if streamSupported {
				toolCalls, content, finish, u, serr := a.consumeStream(ctx, ch, cb)
				if serr != nil {
					return full.String(), usage, serr
				}
				if u.TotalTokens > 0 {
					usage = u
				}

				if len(toolCalls) > 0 {
					for _, tc := range toolCalls {
						a.processToolResult(ctx, tc, cb, &toolCount, &functionResults, &messages)
					}
					continue
				}

				if content != "" {
					full.WriteString(content)
					messages = append(messages, llm.Message{Role: "assistant", Content: content})
				}
				if finish != "" && finish != "tool_calls" {
					break
				}
				if len(toolCalls) == 0 {
					break
				}
			}
		}

		if !streamSupported {
			resp, err := a.adapter.Chat(ctx, req)
			if err != nil {
				return full.String(), usage, fmt.Errorf("chat error: %w", err)
			}
			if resp.Usage.TotalTokens > 0 {
				usage = resp.Usage
			}
			if len(resp.Choices) == 0 {
				return full.String(), usage, fmt.Errorf("empty response from LLM")
			}

			choice := resp.Choices[0]
			messages = append(messages, choice.Message)

			if len(choice.ToolCalls) > 0 {
				for _, tc := range choice.ToolCalls {
					a.processToolResult(ctx, tc, cb, &toolCount, &functionResults, &messages)
				}
				continue
			}

			full.WriteString(choice.Message.Content)
			cb(StreamEvent{Kind: "chunk", Content: choice.Message.Content})
			break
		}
	}

	// Persist to memory
	resultData := map[string]any{
		"agent":       a.ID,
		"agentName":   a.Name,
		"model":       model,
		"prompt":      prompt,
		"response":    full.String(),
		"tokens":      usage.TotalTokens,
		"toolCount":   toolCount,
	}
	if len(functionResults) > 0 {
		resultData["toolResults"] = functionResults
	}

	if a.store != nil {
		meta := memory.Metadata{AgentID: a.ID, Kind: "agent-state", Tags: []string{"llm", "result", "stream"}}
		memPath := fmt.Sprintf("agents/%s/results/%s.json", a.ID, time.Now().Format("20060102-150405"))
		data, err := json.MarshalIndent(resultData, "", "  ")
		if err != nil {
			// Log but don't fail the stream on marshal error
			fmt.Printf("agent %s: marshal stream result: %v\n", a.ID[:8], err)
		} else {
			if err := a.store.Put(memPath, data, meta); err != nil {
				fmt.Printf("agent %s: store stream result: %v\n", a.ID[:8], err)
			}
		}
	}

	cb(StreamEvent{
		Kind:             "done",
		Model:            model,
		Content:          full.String(),
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	})

	return full.String(), usage, nil
}

func (a *Agent) consumeStream(ctx context.Context, ch <-chan llm.StreamChunk, cb StreamCallback) ([]llm.ToolCall, string, string, llm.Usage, error) {
	var content strings.Builder
	var toolCalls []llm.ToolCall
	var finish string
	var usage llm.Usage

	for {
		select {
		case <-ctx.Done():
			return toolCalls, content.String(), finish, usage, ctx.Err()
		case chunk, ok := <-ch:
			if !ok {
				return toolCalls, content.String(), finish, usage, nil
			}
			if chunk.Error != "" {
				cb(StreamEvent{Kind: "error", Error: chunk.Error})
				return toolCalls, content.String(), finish, usage, fmt.Errorf("stream: %s", chunk.Error)
			}
			if chunk.Content != "" {
				content.WriteString(chunk.Content)
				cb(StreamEvent{Kind: "chunk", Content: chunk.Content})
			}
			if len(chunk.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.ToolCalls...)
			}
			if chunk.Finish != "" {
				finish = chunk.Finish
			}
			if chunk.Usage.TotalTokens > 0 {
				usage = chunk.Usage
			}
			if chunk.Done {
				return toolCalls, content.String(), finish, usage, nil
			}
		}
	}
}

func (a *Agent) processToolResult(ctx context.Context, tc llm.ToolCall, cb StreamCallback, toolCount *int, functionResults *[]map[string]any, messages *[]llm.Message) {
	argsStr := string(tc.Arguments)
	var toolArgs map[string]any
	_ = json.Unmarshal(tc.Arguments, &toolArgs)
	if toolArgs == nil {
		toolArgs = map[string]any{"raw": argsStr}
	}

	cb(StreamEvent{Kind: "tool_call", Tool: tc.Name, Args: argsStr, Content: fmt.Sprintf("Calling %s...", tc.Name)})

	if a.registry != nil {
		result, execErr := a.registry.Execute(ctx, a.enforcer, a.Capability, tc.Name, toolArgs)
		*toolCount++
		if execErr != nil {
			*functionResults = append(*functionResults, map[string]any{"error": execErr.Error()})
			cb(StreamEvent{Kind: "tool_result", Tool: tc.Name, Error: execErr.Error(), Content: fmt.Sprintf("%s error", tc.Name)})
		} else {
			*functionResults = append(*functionResults, result)
			cb(StreamEvent{Kind: "tool_result", Tool: tc.Name, Content: fmt.Sprintf("%s done", tc.Name)})

			toolResultBytes, err := json.Marshal(result)
			if err != nil {
				// If marshaling fails, send error to LLM
				*messages = append(*messages, llm.Message{
					Role:    "tool",
					Content: fmt.Sprintf(`{"error": "marshal tool result: %s"}`, err.Error()),
					Name:    tc.Name,
				})
			} else {
				*messages = append(*messages, llm.Message{Role: "tool", Content: string(toolResultBytes), Name: tc.Name})
			}
		}
	}
}