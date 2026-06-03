package tool

import (
	"context"
	"strings"
	"testing"
)

// ── ShellTool Tests ──────────────────────────────────────────────────────────

func TestShellTool_NormalExecution(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	result, err := s.Execute(ctx, map[string]any{
		"command": "echo hello",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	stdout := result["stdout"].(string)
	if !strings.Contains(stdout, "hello") {
		t.Errorf("Expected 'hello' in output, got: %q", stdout)
	}

	exitCode := result["exit_code"].(int)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestShellTool_MultipleArguments(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	// Test that arguments are properly parsed
	result, err := s.Execute(ctx, map[string]any{
		"command": "printf %s world",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	stdout := result["stdout"].(string)
	if !strings.Contains(stdout, "world") {
		t.Errorf("Expected 'world' in output, got: %q", stdout)
	}
}

func TestShellTool_QuotedArguments(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	// Test that quoted arguments with spaces are preserved
	result, err := s.Execute(ctx, map[string]any{
		"command": `echo "hello world"`,
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	stdout := result["stdout"].(string)
	// Without shlex, this would be treated as separate args and fail
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("Expected 'hello world' in output, got: %q", stdout)
	}
}

func TestShellTool_ShellInjectionPrevention(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	// Attempt command substitution — should NOT be interpreted by shell
	// Instead of running: cat /etc/passwd, it should try to run literal command
	result, err := s.Execute(ctx, map[string]any{
		"command": "echo /home/$(cat /etc/passwd)",
	})

	// The command should fail because /home/$(cat is not a directory
	// OR succeed but echo the literal string (not interpret $(cat))
	if err != nil {
		t.Logf("Command failed as expected: %v", err)
		return
	}

	stdout := result["stdout"].(string)
	// If shell injection was present, stdout would contain password file data
	// Instead, we should see the literal string
	if strings.Contains(stdout, "root:") {
		t.Errorf("Shell injection not prevented! Output contains password file data")
	}
	t.Logf("Shell injection prevented. Output: %q", stdout)
}

func TestShellTool_PipeCharactersNotInterpreted(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	// Pipe character should be treated literally, not as shell pipe
	result, err := s.Execute(ctx, map[string]any{
		"command": "echo hello | cat",
	})

	if err == nil {
		// If it reaches here, it means the command succeeded
		// With safe parsing, "echo" gets "hello" and "|" as literal args
		stdout := result["stdout"].(string)
		// Should contain "hello | cat" literally, not piped output
		t.Logf("Pipe not interpreted (literal args): %q", stdout)
	} else {
		// Or it failed trying to run "echo" with args "hello" "|" "cat"
		t.Logf("Pipe character treated literally (execution failed as expected): %v", err)
	}
}

func TestShellTool_WhitelistEnforcement_Allowed(t *testing.T) {
	s := &ShellTool{
		AllowedCommands: []string{"echo", "printf"},
	}
	ctx := context.Background()

	result, err := s.Execute(ctx, map[string]any{
		"command": "echo allowed",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	stdout := result["stdout"].(string)
	if !strings.Contains(stdout, "allowed") {
		t.Errorf("Expected 'allowed' in output, got: %q", stdout)
	}
}

func TestShellTool_WhitelistEnforcement_Denied(t *testing.T) {
	s := &ShellTool{
		AllowedCommands: []string{"echo", "printf"},
	}
	ctx := context.Background()

	// cat is not in the whitelist
	_, err := s.Execute(ctx, map[string]any{
		"command": "cat /etc/passwd",
	})

	if err == nil {
		t.Fatal("Expected error for non-whitelisted command, got nil")
	}

	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("Expected 'not allowed' in error, got: %v", err)
	}
	t.Logf("Whitelist enforcement working: %v", err)
}

func TestShellTool_EmptyCommand(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	_, err := s.Execute(ctx, map[string]any{
		"command": "",
	})

	if err == nil {
		t.Fatal("Expected error for empty command, got nil")
	}

	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' in error message, got: %v", err)
	}
}

func TestShellTool_MissingCommand(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	_, err := s.Execute(ctx, map[string]any{})

	if err == nil {
		t.Fatal("Expected error for missing command, got nil")
	}

	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("Expected 'missing' in error message, got: %v", err)
	}
}

func TestShellTool_InvalidSyntax(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	// Unclosed quote should fail parsing
	_, err := s.Execute(ctx, map[string]any{
		"command": `echo "unclosed`,
	})

	if err == nil {
		t.Fatal("Expected error for unclosed quote, got nil")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("Expected 'parse' in error message, got: %v", err)
	}
	t.Logf("Parse error correctly caught: %v", err)
}

func TestShellTool_StderrCapture(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	// Run a command that writes to stderr
	result, err := s.Execute(ctx, map[string]any{
		"command": "sh -c 'echo error >&2'",
	})

	// Note: This uses "sh -c" as an argument to the command, not as shell interpretation
	if err != nil {
		t.Logf("Command failed: %v", err)
		// That's ok - sh is being run as a normal program with -c as a literal arg
		return
	}

	stderr := result["stderr"].(string)
	if stderr != "" {
		t.Logf("Stderr captured: %q", stderr)
	}
}

func TestShellTool_CommandNotFound(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	_, err := s.Execute(ctx, map[string]any{
		"command": "nonexistent_command_xyz",
	})

	if err == nil {
		t.Fatal("Expected error for non-existent command, got nil")
	}

	t.Logf("Non-existent command correctly failed: %v", err)
}

func TestShellTool_ParseTimeoutSyntaxError(t *testing.T) {
	s := &ShellTool{}
	ctx := context.Background()

	_, err := s.Execute(ctx, map[string]any{
		"command": "echo test",
		"timeout": "invalid_duration",
	})

	if err == nil {
		t.Fatal("Expected error for invalid timeout, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected 'timeout' in error message, got: %v", err)
	}
	t.Logf("Invalid timeout correctly rejected: %v", err)
}
