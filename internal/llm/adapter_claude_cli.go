// adapter_claude_cli.go — Claude CLI adapter (spawns local claude command)

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

type ClaudeCliAdapter struct {
	cliPath string
	model   string
	timeout time.Duration
}

func NewClaudeCLI(cliPath, model string, timeout time.Duration) Adapter {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &ClaudeCliAdapter{cliPath, model, timeout}
}

func (c *ClaudeCliAdapter) Provider() string { return "claude-cli" }

func (c *ClaudeCliAdapter) ContextWindow() int {
	if strings.Contains(c.model, "haiku") {
		return 200000
	}
	return 200000
}

func (c *ClaudeCliAdapter) Chat(ctx context.Context, req Request) (Response, error) {
	prompt := buildPrompt(req.Messages)
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	args := []string{"-p", prompt}
	if model != "" {
		args = append(args, "--model", model)
	}
	cmd := exec.CommandContext(ctx, c.cliPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return Response{}, fmt.Errorf("claude cli: %w", err)
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

func (c *ClaudeCliAdapter) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 10)
	prompt := buildPrompt(req.Messages)

	timeout := c.timeout
	if timeout < 5*time.Minute {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)

	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	go func() {
		defer close(ch)
		defer cancel()

		args := []string{"-p", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
		cmd := exec.CommandContext(ctx, c.cliPath, args...)
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
			ch <- StreamChunk{Error: fmt.Sprintf("failed to start claude: %v", err), Done: true}
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
			// If we already received content, treat as partial success.
			if content.Len() == 0 {
				errDetail := strings.TrimSpace(errBuf.String())
				if errDetail != "" {
					ch <- StreamChunk{Error: fmt.Sprintf("claude cli: %v — %s", err, errDetail), Done: true}
				} else {
					ch <- StreamChunk{Error: fmt.Sprintf("claude cli: %v", err), Done: true}
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

func (c *ClaudeCliAdapter) HealthCheck(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.cliPath, "--version")
	return cmd.Run()
}

func buildPrompt(messages []Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
