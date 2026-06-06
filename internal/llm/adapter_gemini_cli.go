// adapter_gemini_cli.go — Gemini CLI adapter (spawns local gemini command)

package llm

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type GeminiCliAdapter struct {
	cliPath string
	model   string
	timeout time.Duration
}

func NewGeminiCLI(cliPath, model string, timeout time.Duration) Adapter {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &GeminiCliAdapter{cliPath, model, timeout}
}

func (g *GeminiCliAdapter) Provider() string { return "gemini-cli" }

func (g *GeminiCliAdapter) ContextWindow() int {
	switch g.model {
	case "gemini-2.5-pro", "gemini-2.5-flash":
		return 1000000
	case "gemini-2.0-flash", "gemini-2.0-flash-thinking-exp":
		return 1000000
	case "gemini-1.5-pro", "gemini-1.5-flash":
		return 1000000
	default:
		return 32000
	}
}

func (g *GeminiCliAdapter) Chat(ctx context.Context, req Request) (Response, error) {
	prompt := buildPrompt(req.Messages)
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	model := g.model
	if req.Model != "" {
		model = req.Model
	}

	args := []string{"--prompt", prompt}
	if model != "" {
		args = append(args, "-m", model)
	}
	cmd := exec.CommandContext(ctx, g.cliPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return Response{}, fmt.Errorf("gemini cli: %w", err)
	}

	content := strings.TrimSpace(string(output))
	inputTokens := len(prompt) / 4
	outputTokens := len(content) / 4

	return Response{
		Model: model,
		Choices: []Choice{{
			Index:  0,
			Message: Message{Role: "assistant", Content: content},
			Finish: "stop",
		}},
		Usage: Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
	}, nil
}

func (g *GeminiCliAdapter) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 10)
	prompt := buildPrompt(req.Messages)

	timeout := g.timeout
	if timeout < 5*time.Minute {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)

	model := g.model
	if req.Model != "" {
		model = req.Model
	}

	go func() {
		defer close(ch)
		defer cancel()

		args := []string{"--prompt", prompt}
		if model != "" {
			args = append(args, "-m", model)
		}
		cmd := exec.CommandContext(ctx, g.cliPath, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			ch <- StreamChunk{Error: fmt.Sprintf("failed to pipe stdout: %v", err), Done: true}
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			ch <- StreamChunk{Error: fmt.Sprintf("failed to pipe stderr: %v", err), Done: true}
			return
		}

		if err := cmd.Start(); err != nil {
			ch <- StreamChunk{Error: fmt.Sprintf("failed to start gemini: %v", err), Done: true}
			return
		}

		// Drain stderr concurrently — prevents pipe-buffer deadlock when the
		// CLI writes substantial stderr output while also writing to stdout.
		var errBuf strings.Builder
		var errWg sync.WaitGroup
		errWg.Add(1)
		go func() {
			defer errWg.Done()
			errScanner := bufio.NewScanner(stderr)
			for errScanner.Scan() {
				errBuf.WriteString(errScanner.Text())
				errBuf.WriteString("\n")
			}
		}()

		var content strings.Builder
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			content.WriteString(line)
			content.WriteString("\n")
			ch <- StreamChunk{Model: model, Content: line, Role: "assistant"}
		}

		errWg.Wait()

		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			// If we already received content, treat as partial success rather than error.
			// This handles cases where the CLI writes a valid response then exits non-zero
			// (e.g. due to a failed tool call on stderr that doesn't affect the LLM output).
			if content.Len() == 0 {
				errDetail := strings.TrimSpace(errBuf.String())
				if errDetail != "" {
					ch <- StreamChunk{Error: fmt.Sprintf("gemini cli: %v — %s", err, errDetail), Done: true}
				} else {
					ch <- StreamChunk{Error: fmt.Sprintf("gemini cli: %v", err), Done: true}
				}
				return
			}
		}

		fullContent := strings.TrimSpace(content.String())
		inputTokens := len(prompt) / 4
		outputTokens := len(fullContent) / 4

		ch <- StreamChunk{
			Done: true,
			Usage: Usage{
				PromptTokens:     inputTokens,
				CompletionTokens: outputTokens,
				TotalTokens:      inputTokens + outputTokens,
			},
		}
	}()

	return ch, nil
}

func (g *GeminiCliAdapter) HealthCheck(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, g.cliPath, "--version")
	return cmd.Run()
}
