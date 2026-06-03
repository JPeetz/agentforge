package tool

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatToolResult_Success(t *testing.T) {
	result := map[string]any{"status": "ok", "file": "test.txt", "size": 42}
	rf := FormatToolResult("read_file", result, nil)
	if rf.ForLLM == "" {
		t.Error("ForLLM must not be empty")
	}
	if rf.ForUser == "" {
		t.Error("ForUser must not be empty")
	}
	if !strings.Contains(rf.ForUser, "read_file") {
		t.Error("ForUser should mention tool name")
	}
}

func TestFormatToolResult_Error(t *testing.T) {
	rf := FormatToolResult("shell_exec", nil, errors.New("permission denied"))
	if !strings.Contains(rf.ForUser, "❌") {
		t.Error("error result should contain ❌")
	}
	if !strings.Contains(rf.ForLLM, "permission denied") {
		t.Error("ForLLM should include error text")
	}
}