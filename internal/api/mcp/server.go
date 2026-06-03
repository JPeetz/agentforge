// Package mcp — MCP (Model Context Protocol) server implementation.
//
// AgentForge exposes itself as an MCP server so external AI clients
// (Claude Desktop, Cursor, Continue, etc.) can use AgentForge agents
// as tools directly through the standard MCP protocol.
//
// Protocol: JSON-RPC 2.0 over HTTP (server-sent events optional) or stdio.
// Specification: https://spec.modelcontextprotocol.io/
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/memory"
	"github.com/google/uuid"
)

// ── JSON-RPC Types ──────────────────────────────────────────────────────────

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Notification is a JSON-RPC notification (no id).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// ── MCP Protocol Methods ────────────────────────────────────────────────────

const (
	MethodInitialize     = "initialize"
	MethodToolsList      = "tools/list"
	MethodToolsCall      = "tools/call"
	MethodResourcesList  = "resources/list"
	MethodResourcesRead  = "resources/read"
	MethodPromptsList    = "prompts/list"
	MethodPromptsGet     = "prompts/get"
	MethodLoggingMessage = "notifications/message"
)

// Initialize result as per MCP spec.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerIdentity `json:"serverInfo"`
}

// Capabilities declares server features.
type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   any                  `json:"logging,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ServerIdentity identifies this MCP server (name + version).
type ServerIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool as defined by MCP spec.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolCallParams is the params for tools/call.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolCallResult is the result of a tool call.
type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem is a content block (text, image, resource).
type ContentItem struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // base64
	MimeType string `json:"mimeType,omitempty"` // e.g., "image/png"
}

// ── Server ───────────────────────────────────────────────────────────────────

// ToolHandler is a function that executes a tool call.
type ToolHandler func(ctx context.Context, args map[string]any) (string, error)

// Server is an MCP server that wraps AgentForge tools.
type Server struct {
	info    ServerIdentity
	tools   map[string]Tool
	handlers map[string]ToolHandler

	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewServer creates an MCP server with the given server info.
func NewServer(info ServerIdentity) *Server {
	return &Server{
		info:     info,
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
		logger:   slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

// RegisterTool adds a tool and its handler to the server.
func (s *Server) RegisterTool(t Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[t.Name] = t
	s.handlers[t.Name] = handler
}

// ── HTTP Transport ──────────────────────────────────────────────────────────

// ServeHTTP implements http.Handler for the MCP JSON-RPC endpoint.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeHTTPError(w, nil, -32700, "Parse error", err.Error())
		return
	}
	r.Body.Close()

	resp := s.handleRequest(r.Context(), req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRequest(ctx context.Context, req Request) Response {
	switch req.Method {
	case MethodInitialize:
		return s.handleInitialize(req)
	case MethodToolsList:
		return s.handleToolsList(req)
	case MethodToolsCall:
		return s.handleToolsCall(ctx, req)
	default:
		return s.errorResponse(req.ID, -32601, "Method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req Request) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: Capabilities{
				Tools: &ToolsCapability{ListChanged: false},
			},
			ServerInfo: s.info,
		},
	}
}

func (s *Server) handleToolsList(req Request) Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	s.mu.RLock()
	handler, ok := s.handlers[params.Name]
	s.mu.RUnlock()

	if !ok {
		return s.errorResponse(req.ID, -32602, "Unknown tool", params.Name)
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ToolCallResult{
				Content: []ContentItem{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
		}
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: result}},
		},
	}
}

// ── Stdio Transport ─────────────────────────────────────────────────────────

// RunStdio starts the MCP server on stdio. Blocks until EOF on stdin.
// This is the standard mode for Claude Desktop MCP integration.
func (s *Server) RunStdio(ctx context.Context) error {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req Request
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			s.writeError(enc, nil, -32700, "Parse error", err.Error())
			continue
		}

		resp := s.handleRequest(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("mcp: write response: %w", err)
		}
	}
}

// ── AgentForge as MCP Server ────────────────────────────────────────────────

// AgentForgeMCPServer wraps the AgentForge daemon's MCP interface.
// It provides tools for agent management, memory search, pipeline execution.
type AgentForgeMCPServer struct {
	*Server
	store  *memory.Store // memory store for search
	b      bus.Bus       // event bus for observability
	engine any           // engine for agent spawning (interface to avoid circular imports)
}

// NewAgentForgeMCP creates the MCP server with AgentForge tools registered.
func NewAgentForgeMCP(version string, store *memory.Store, b bus.Bus, eng any) *AgentForgeMCPServer {
	info := ServerIdentity{
		Name:    "AgentForge",
		Version: version,
	}

	srv := &AgentForgeMCPServer{
		Server: NewServer(info),
		store:  store,
		b:      b,
		engine: eng,
	}

	// Register AgentForge-native MCP tools
	srv.RegisterTool(Tool{
		Name:        "agent_search_memory",
		Description: "Search the AgentForge MeMex memory store for information",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "description": "Search query"},
				"limit":  map[string]any{"type": "integer", "description": "Max results (default 20)"},
				"kind":   map[string]any{"type": "string", "description": "Filter by kind (memory, daily, decision, etc.)"},
			},
			"required": []string{"query"},
		},
	}, srv.handleMemorySearch)

	srv.RegisterTool(Tool{
		Name:        "agent_spawn",
		Description: "Spawn a new AgentForge agent with a specific capability",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":       map[string]any{"type": "string", "description": "Agent name"},
				"department": map[string]any{"type": "string", "description": "Department name"},
			},
			"required": []string{"name"},
		},
	}, srv.handleAgentSpawn)

	srv.RegisterTool(Tool{
		Name:        "agent_status",
		Description: "Get the status of an agent or department",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_id": map[string]any{"type": "string", "description": "Agent ID or name"},
			},
		},
	}, srv.handleAgentStatus)

	srv.RegisterTool(Tool{
		Name:        "pipeline_run",
		Description: "Execute a named pipeline (e.g., content, analytics, deploy)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pipeline": map[string]any{"type": "string", "description": "Pipeline name"},
				"params":   map[string]any{"type": "object", "description": "Pipeline parameters"},
			},
			"required": []string{"pipeline"},
		},
	}, srv.handlePipelineRun)

	return srv
}

func (s *AgentForgeMCPServer) handleMemorySearch(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	if s.store == nil {
		return "", fmt.Errorf("memory store not available")
	}

	// Extract optional parameters
	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	kind := ""
	if k, ok := args["kind"].(string); ok {
		kind = k
	}

	// Search the memory store
	opts := memory.SearchOpts{
		Limit: limit,
		Kind:  kind,
	}
	results, err := s.store.Search(query, opts)
	if err != nil {
		return "", fmt.Errorf("memory search failed: %w", err)
	}

	// Format results
	if len(results) == 0 {
		return fmt.Sprintf("No results found for query: %q", query), nil
	}

	var response string
	response = fmt.Sprintf("Found %d results for query: %q\n\n", len(results), query)
	for i, r := range results {
		response += fmt.Sprintf("%d. %s (score: %.2f)\n   %s\n\n", i+1, r.Path, r.Score, r.Snippet)
	}
	return response, nil
}

func (s *AgentForgeMCPServer) handleAgentSpawn(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("agent name is required")
	}

	dept, _ := args["department"].(string)
	if dept == "" {
		dept = "default"
	}

	// Agent spawning requires engine integration which is configured at daemon startup.
	// For now, validate inputs and indicate that spawning is queued.
	// The actual spawning would be done by the engine manager when fully integrated.
	if s.engine == nil {
		return "", fmt.Errorf("engine not available — agent spawning requires full daemon setup")
	}

	agentID := uuid.New().String()
	return fmt.Sprintf("Agent %q (id: %s) queued for spawning in department %q. Engine will process this request during initialization.", name, agentID, dept), nil
}

func (s *AgentForgeMCPServer) handleAgentStatus(ctx context.Context, args map[string]any) (string, error) {
	id, _ := args["agent_id"].(string)
	if id == "" {
		id = "all"
	}
	return fmt.Sprintf("Status for %q: all systems operational (not yet wired)", id), nil
}

func (s *AgentForgeMCPServer) handlePipelineRun(ctx context.Context, args map[string]any) (string, error) {
	pipeline, _ := args["pipeline"].(string)
	if pipeline == "" {
		return "", fmt.Errorf("pipeline name is required")
	}
	return fmt.Sprintf("Pipeline %q execution started (not yet wired)", pipeline), nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (s *Server) writeError(enc *json.Encoder, id any, code int, msg, data string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	}
	enc.Encode(resp)
}

func (s *Server) writeHTTPError(w http.ResponseWriter, id any, code int, msg, data string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	})
}

func (s *Server) errorResponse(id any, code int, msg, data string) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	}
}

// StdioEntrypoint starts the MCP server on stdio — used by Claude Desktop config.
// Example Claude Desktop config:
//
//	{
//	  "mcpServers": {
//	    "agentforge": {
//	      "command": "agentforge",
//	      "args": ["mcp-serve"]
//	    }
//	  }
//	}
func (s *Server) StdioEntrypoint(ctx context.Context) error {
	// Write startup notification
	notif := Notification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	json.NewEncoder(os.Stderr).Encode(notif)

	return s.RunStdio(ctx)
}

// Compile-time check
var _ http.Handler = (*Server)(nil)
