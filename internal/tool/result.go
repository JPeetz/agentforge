// Package tool — tool result formatting.
package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResultFormat provides dual-channel tool output:
// ForLLM is compact JSON for the LLM context (token-efficient).
// ForUser is rich Markdown for display to the human user.
type ResultFormat struct {
	ForLLM  string `json:"for_llm"`
	ForUser string `json:"for_user"`
}

// FormatToolResult takes a raw tool execution result and produces
// both channels. Each tool category gets a custom renderer.
func FormatToolResult(toolName string, rawResult map[string]any, execErr error) ResultFormat {
	if execErr != nil {
		errJSON, _ := json.Marshal(map[string]any{"error": execErr.Error()})
		return ResultFormat{
			ForLLM:  string(errJSON),
			ForUser: fmt.Sprintf("❌ **%s failed:** %s", toolName, execErr.Error()),
		}
	}
	forLLM, _ := json.Marshal(rawResult)
	forUser := renderForUser(toolName, rawResult)
	return ResultFormat{
		ForLLM:  string(forLLM),
		ForUser: forUser,
	}
}

func renderForUser(toolName string, result map[string]any) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s\n\n", toolName))

	// Ordered rendering based on common keys
	ordered := []string{"status", "file", "path", "content", "result", "output", "error", "count", "size", "url", "message"}

	for _, key := range ordered {
		if val, ok := result[key]; ok {
			b.WriteString(fmt.Sprintf("**%s:** %v\n", key, val))
		}
	}

	// Any unrendered keys
	for key, val := range result {
		found := false
		for _, k := range ordered {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			s, _ := json.MarshalIndent(val, "", "  ")
			b.WriteString(fmt.Sprintf("**%s:** %s\n", key, string(s)))
		}
	}
	return b.String()
}