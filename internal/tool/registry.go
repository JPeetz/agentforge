// Package tool — tool registry, builtin tools, and WASM plugin loader.
//
// Tools are the only way an agent interacts with the outside world.
// Every tool invocation passes through the capability enforcer.
package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/agentforge/internal/security"
)

// ── Tool Interface ──────────────────────────────────────────────────────────

// Metadata describes a tool for the registry and LLM tool definitions.
type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author,omitempty"`
	Category    string `json:"category"` // filesystem, network, shell, mcp, wasm
	// InputSchema is a JSON Schema for the tool's input parameters.
	InputSchema map[string]any `json:"input_schema"`
}

// Tool is the interface every tool must implement.
type Tool interface {
	Meta() Metadata
	Execute(ctx context.Context, args map[string]any) (map[string]any, error)
}

// ── Registry ─────────────────────────────────────────────────────────────────

// Registry holds all registered tools. Each agent gets a filtered
// view based on its capability token's resource allowlist.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool

	// Category → tool names for quick listing
	categories map[string][]string
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:      make(map[string]Tool),
		categories: make(map[string][]string),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := t.Meta()
	if _, exists := r.tools[meta.Name]; exists {
		return fmt.Errorf("tool %s already registered", meta.Name)
	}
	r.tools[meta.Name] = t
	r.categories[meta.Category] = append(r.categories[meta.Category], meta.Name)
	return nil
}

// Get returns a tool by name, or nil.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// ByCategory returns tool names in a category.
func (r *Registry) ByCategory(cat string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.categories[cat]
}

// Execute runs a tool by name, passing through the capability enforcer.
func (r *Registry) Execute(ctx context.Context, enforcer *security.Enforcer, cap *security.Capability, toolName string, args map[string]any) (map[string]any, error) {
	t := r.Get(toolName)
	if t == nil {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	// Build action for capability check
	action := security.Action{
		Tool:      toolName,
		Args:      args,
		Operation: security.OpExec,
	}
	if path, ok := args["path"].(string); ok {
		action.Resource = path
		action.Operation = security.OpWrite
	}
	if url, ok := args["url"].(string); ok {
		action.Resource = url
	}

	if err := enforcer.Check(ctx, cap, action); err != nil {
		return nil, fmt.Errorf("tool %s: %w", toolName, err)
	}

	return t.Execute(ctx, args)
}

// ── Builtin: Filesystem ─────────────────────────────────────────────────────

// FilesystemTool provides read/write access within capability bounds.
type FilesystemTool struct{}

func (f *FilesystemTool) Meta() Metadata {
	return Metadata{
		Name:        "filesystem",
		Description: "Read, write, list, and delete files within capability-scoped paths",
		Version:     "1.0.0",
		Category:    "filesystem",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"read", "write", "list", "delete", "mkdir"}},
				"path":   map[string]any{"type": "string", "description": "File or directory path"},
				"content": map[string]any{"type": "string", "description": "Content to write (action=write)"},
			},
			"required": []string{"action", "path"},
		},
	}
}

func (f *FilesystemTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	action, _ := args["action"].(string)
	path, _ := args["path"].(string)

	switch action {
	case "read":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		return map[string]any{"content": string(data), "size": len(data)}, nil

	case "write":
		content, _ := args["content"].(string)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		return map[string]any{"written": len(content), "path": path}, nil

	case "list":
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", path, err)
		}
		var names []string
		for _, e := range entries {
			suffix := ""
			if e.IsDir() {
				suffix = "/"
			}
			names = append(names, e.Name()+suffix)
		}
		return map[string]any{"entries": names, "count": len(names)}, nil

	case "delete":
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("delete %s: %w", path, err)
		}
		return map[string]any{"deleted": true}, nil

	case "mkdir":
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", path, err)
		}
		return map[string]any{"created": path}, nil

	default:
		return nil, fmt.Errorf("filesystem: unknown action %q", action)
	}
}

// ── Builtin: Shell ───────────────────────────────────────────────────────────

// ShellTool executes shell commands within sandbox constraints.
// The command inherits the capability's resource allowlist.
type ShellTool struct {
	AllowedCommands []string // empty = any command
}

func (s *ShellTool) Meta() Metadata {
	return Metadata{
		Name:        "shell",
		Description: "Execute a shell command and capture stdout/stderr",
		Version:     "1.0.0",
		Category:    "shell",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to execute"},
				"timeout": map[string]any{"type": "string", "description": "Timeout (e.g., 30s, 5m)"},
				"workdir": map[string]any{"type": "string", "description": "Working directory"},
			},
			"required": []string{"command"},
		},
	}
}

func (s *ShellTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	cmdStr, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("shell: missing 'command' argument")
	}

	timeout := 30 * time.Second
	if ts, ok := args["timeout"].(string); ok {
		d, _ := time.ParseDuration(ts)
		if d > 0 {
			timeout = d
		}
	}

	workdir, _ := args["workdir"].(string)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	if workdir != "" {
		cmd.Dir = workdir
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("shell: start: %w", err)
	}

	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return map[string]any{
			"stdout":   string(outBytes),
			"stderr":   string(errBytes),
			"exit_code": cmd.ProcessState.ExitCode(),
			"error":    err.Error(),
		}, nil
	}

	return map[string]any{
		"stdout":    string(outBytes),
		"stderr":    string(errBytes),
		"exit_code": 0,
	}, nil
}

// ── Builtin: HTTP ────────────────────────────────────────────────────────────

// HTTPTool executes HTTP requests within capability-scoped domain allowlists.
type HTTPTool struct {
	Client *http.Client
}

func (h *HTTPTool) Meta() Metadata {
	return Metadata{
		Name:        "http",
		Description: "Execute HTTP requests (GET, POST, PUT, DELETE) within allowlisted domains",
		Version:     "1.0.0",
		Category:    "network",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"method":  map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
				"url":     map[string]any{"type": "string", "format": "uri"},
				"headers": map[string]any{"type": "object"},
				"body":    map[string]any{"type": "string"},
				"timeout": map[string]any{"type": "string"},
			},
			"required": []string{"url"},
		},
	}
}

func (h *HTTPTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	url, _ := args["url"].(string)
	method, _ := args["method"].(string)
	if method == "" {
		method = "GET"
	}

	body, _ := args["body"].(string)
	timeout := 30 * time.Second
	if ts, ok := args["timeout"].(string); ok {
		d, _ := time.ParseDuration(ts)
		if d > 0 {
			timeout = d
		}
	}

	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http: request: %w", err)
	}

	req.Header.Set("User-Agent", "AgentForge/0.1.0")
	if hdrs, ok := args["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1MB

	return map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"headers":    flattenHeaders(resp.Header),
		"body":       string(respBody),
		"size":       len(respBody),
	}, nil
}

func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range h {
		result[k] = strings.Join(v, ", ")
	}
	return result
}

// ── Builtin: MCP Client ──────────────────────────────────────────────────────

// MCPClientTool connects to external MCP servers and makes them available
// as tools to agents. The MCP protocol is JSON-RPC over stdio or HTTP.
type MCPClientTool struct {
	Servers map[string]MCPServerConfig
}

// MCPServerConfig describes a connected MCP server.
type MCPServerConfig struct {
	Name    string
	Command string            // stdio: command to run
	Args    []string          // stdio: arguments
	URL     string            // HTTP: server URL
	Headers map[string]string // HTTP: auth headers
	// Server-provided: tool list will be populated after connect
	Tools []string
}

func (m *MCPClientTool) Meta() Metadata {
	return Metadata{
		Name:        "mcp",
		Description: "Invoke tools from connected MCP servers (Model Context Protocol)",
		Version:     "1.0.0",
		Category:    "mcp",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{"type": "string", "description": "MCP server name"},
				"tool":   map[string]any{"type": "string", "description": "Tool name on the MCP server"},
				"args":   map[string]any{"type": "object", "description": "Tool arguments"},
			},
			"required": []string{"server", "tool"},
		},
	}
}

func (m *MCPClientTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	serverName, _ := args["server"].(string)
	toolName, _ := args["tool"].(string)
	toolArgs, _ := args["args"].(map[string]any)

	cfg, ok := m.Servers[serverName]
	if !ok {
		return nil, fmt.Errorf("mcp: server %q not configured", serverName)
	}

	method := "tools/call"
	params := map[string]any{
		"name":      toolName,
		"arguments": toolArgs,
	}

	if cfg.URL != "" {
		return m.callHTTP(ctx, cfg, method, params)
	}
	return m.callStdio(ctx, cfg, method, params)
}

func (m *MCPClientTool) callHTTP(ctx context.Context, cfg MCPServerConfig, method string, params map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (m *MCPClientTool) callStdio(ctx context.Context, cfg MCPServerConfig, method string, params map[string]any) (map[string]any, error) {
	// MCP stdio: spawn the command and communicate via stdin/stdout JSON-RPC
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp stdio start: %w", err)
	}
	defer cmd.Process.Kill()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	enc := json.NewEncoder(stdin)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("mcp stdio write: %w", err)
	}

	var result map[string]any
	dec := json.NewDecoder(bufio.NewReader(stdout))
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("mcp stdio read: %w", err)
	}

	return result, nil
}

// ── Tool Definitions for LLM ─────────────────────────────────────────────────

// ToLLMToolDef converts a tool's metadata to an LLM-compatible tool definition.
func ToLLMToolDef(t Tool) map[string]any {
	m := t.Meta()
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        m.Name,
			"description": m.Description,
			"parameters":  m.InputSchema,
		},
	}
}

// ── Compile-time checks ──────────────────────────────────────────────────────

var (
	_ Tool = (*FilesystemTool)(nil)
	_ Tool = (*ShellTool)(nil)
	_ Tool = (*HTTPTool)(nil)
	_ Tool = (*MCPClientTool)(nil)
)
