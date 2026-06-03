// Package mcpclient — MCP (Model Context Protocol) client implementation.
//
// Enables AgentForge agents to consume tools from external MCP servers
// via JSON-RPC 2.0 over HTTP or stdio transport.
//
// Protocol: https://spec.modelcontextprotocol.io/
package mcpclient

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ── Client Interface ─────────────────────────────────────────────────────────

// ToolDef describes a tool discovered from an MCP server.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// MCPClient is the interface every MCP transport client implements.
type MCPClient interface {
	// Connect establishes the transport connection and performs MCP initialization.
	Connect() error

	// ListTools discovers available tools from the server via tools/list.
	ListTools() ([]ToolDef, error)

	// CallTool invokes a named tool with arguments via tools/call.
	CallTool(name string, args map[string]any) (map[string]any, error)

	// Close terminates the connection and cleans up resources.
	Close() error

	// Name returns the client's configured server name.
	Name() string
}

// ── JSON-RPC wire types ──────────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ── HTTP Client ──────────────────────────────────────────────────────────────

// HTTPClient connects to MCP servers over HTTP transport.
type HTTPClient struct {
	name    string
	url     string
	headers map[string]string
	client  *http.Client
	reqID   int64
	mu      sync.Mutex
	logger  *slog.Logger
}

// NewHTTPClient creates an MCP client that connects via HTTP POST.
// url is the full MCP endpoint, e.g. "http://localhost:9090".
func NewHTTPClient(name, url string, headers map[string]string) *HTTPClient {
	return &HTTPClient{
		name:    name,
		url:     strings.TrimRight(url, "/"),
		headers: headers,
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

// SetLogger sets the logger for this client.
func (c *HTTPClient) SetLogger(l *slog.Logger) { c.logger = l }

// Name returns the server name.
func (c *HTTPClient) Name() string { return c.name }

// Connect performs MCP initialization handshake over HTTP.
func (c *HTTPClient) Connect() error {
	_, err := c.callRPC("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "AgentForge",
			"version": "0.1.0",
		},
	})
	return err
}

// ListTools calls tools/list and returns discovered tools.
func (c *HTTPClient) ListTools() ([]ToolDef, error) {
	result, err := c.callRPC("tools/list", nil)
	if err != nil {
		return nil, err
	}

	// Parse the "tools" array from the result
	var wrapper struct {
		Tools []ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(result, &wrapper); err != nil {
		return nil, fmt.Errorf("mcp client %q: parse tools/list: %w", c.name, err)
	}
	return wrapper.Tools, nil
}

// CallTool calls tools/call on the remote MCP server.
func (c *HTTPClient) CallTool(name string, args map[string]any) (map[string]any, error) {
	result, err := c.callRPC("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}

	var r map[string]any
	if err := json.Unmarshal(result, &r); err != nil {
		return nil, fmt.Errorf("mcp client %q: parse tool result: %w", c.name, err)
	}
	return r, nil
}

// Close is a no-op for HTTP clients.
func (c *HTTPClient) Close() error {
	return nil
}

func (c *HTTPClient) callRPC(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.reqID++
	id := c.reqID
	c.mu.Unlock()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: marshal: %w", c.name, err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: request: %w", c.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: connect: %w", c.name, err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("mcp client %q: decode response: %w", c.name, err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp client %q: rpc error %d: %s", c.name, rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// ── Stdio Client ─────────────────────────────────────────────────────────────

// StdioClient connects to MCP servers by spawning a subprocess and piping JSON-RPC.
type StdioClient struct {
	name     string
	command  string
	args     []string
	env      map[string]string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	reader   *bufio.Reader
	reqID    int64
	mu       sync.Mutex
	logger   *slog.Logger
}

// NewStdioClient creates an MCP client that communicates over stdio.
func NewStdioClient(name, command string, args []string, env map[string]string) *StdioClient {
	return &StdioClient{
		name:    name,
		command: command,
		args:    args,
		env:     env,
		logger:  slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

// SetLogger sets the logger for this client.
func (c *StdioClient) SetLogger(l *slog.Logger) { c.logger = l }

// Name returns the server name.
func (c *StdioClient) Name() string { return c.name }

// Connect spawns the subprocess and performs MCP initialization.
func (c *StdioClient) Connect() error {
	c.cmd = exec.Command(c.command, c.args...)

	// Use filtered environment (blocks secrets like API keys, tokens, passwords)
	c.cmd.Env = FilterEnvironment(c.env)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp client %q: stdin pipe: %w", c.name, err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp client %q: stdout pipe: %w", c.name, err)
	}
	c.cmd.Stderr = os.Stderr
	c.reader = bufio.NewReader(c.stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp client %q: start: %w", c.name, err)
	}

	// Perform initialization handshake
	_, err = c.callRPC("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "AgentForge",
			"version": "0.1.0",
		},
	})
	return err
}

// ListTools calls tools/list over stdio.
func (c *StdioClient) ListTools() ([]ToolDef, error) {
	result, err := c.callRPC("tools/list", nil)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Tools []ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(result, &wrapper); err != nil {
		return nil, fmt.Errorf("mcp client %q: parse tools/list: %w", c.name, err)
	}
	return wrapper.Tools, nil
}

// CallTool calls tools/call over stdio.
func (c *StdioClient) CallTool(name string, args map[string]any) (map[string]any, error) {
	result, err := c.callRPC("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}

	var r map[string]any
	if err := json.Unmarshal(result, &r); err != nil {
		return nil, fmt.Errorf("mcp client %q: parse tool result: %w", c.name, err)
	}
	return r, nil
}

// Close terminates the subprocess.
func (c *StdioClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

func (c *StdioClient) callRPC(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reqID++
	id := c.reqID

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: marshal: %w", c.name, err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("mcp client %q: write: %w", c.name, err)
	}

	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: read: %w", c.name, err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(line, &rpcResp); err != nil {
		return nil, fmt.Errorf("mcp client %q: decode: %w", c.name, err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp client %q: rpc error %d: %s", c.name, rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// ── MCPClientManager ─────────────────────────────────────────────────────────

// ClientState is a public snapshot of a connected MCP client's state.
type ClientState struct {
	Name      string   `json:"name"`
	URL       string   `json:"url,omitempty"`
	Transport string   `json:"transport"`
	Connected bool     `json:"connected"`
	ToolCount int      `json:"toolCount"`
	Tools     []string `json:"tools,omitempty"`
	LastError string   `json:"lastError,omitempty"`
}

// ClientManager manages MCP client connections and tool discovery.
type ClientManager struct {
	clients     map[string]*clientInstance // name → instance
	toolFilter  map[string][]string        // name → allowlist
	autoConnect map[string]bool            // name → autoConnect

	mu      sync.RWMutex
	logger  *slog.Logger
}

type clientInstance struct {
	cfg          MCPClientServerConfig
	client       MCPClient
	tools        []ToolDef
	toolNames    []string
	connected    bool
	lastError    string
	reconnectCh  chan struct{} // closed to stop reconnection
	healthTicker *time.Ticker
}

// MCPClientServerConfig describes a single MCP client connection.
type MCPClientServerConfig struct {
	Name        string            // unique name for this server
	URL         string            // HTTP endpoint URL
	Transport   string            // "http" or "stdio"
	Command     string            // stdio: subprocess command
	Args        []string          // stdio: arguments
	Env         map[string]string // stdio: extra environment vars
	Headers     map[string]string // HTTP: extra request headers
	ToolFilter  []string          // allowlist of tool names (empty = all)
	AutoConnect bool              // connect automatically on init
}

// NewClientManager creates a new MCP client manager.
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:     make(map[string]*clientInstance),
		toolFilter:  make(map[string][]string),
		autoConnect: make(map[string]bool),
		logger:      slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

// SetLogger sets the logger for the manager.
func (cm *ClientManager) SetLogger(l *slog.Logger) { cm.logger = l }

// Connect establishes a connection to an MCP server and discovers tools.
func (cm *ClientManager) Connect(cfg MCPClientServerConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.clients[cfg.Name]; exists {
		// Already connected — close and reconnect
		cm.disconnectLocked(cfg.Name)
	}

	if cfg.Transport == "" {
		cfg.Transport = "http"
	}

	var client MCPClient
	switch cfg.Transport {
	case "stdio":
		client = NewStdioClient(cfg.Name, cfg.Command, cfg.Args, cfg.Env)
	default:
		client = NewHTTPClient(cfg.Name, cfg.URL, cfg.Headers)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect to %q: %w", cfg.Name, err)
	}

	tools, err := client.ListTools()
	if err != nil {
		client.Close()
		return fmt.Errorf("list tools from %q: %w", cfg.Name, err)
	}

	// Apply tool filter
	filtered := tools
	if len(cfg.ToolFilter) > 0 {
		filterSet := make(map[string]bool, len(cfg.ToolFilter))
		for _, f := range cfg.ToolFilter {
			filterSet[f] = true
		}
		filtered = make([]ToolDef, 0, len(tools))
		for _, t := range tools {
			if filterSet[t.Name] {
				filtered = append(filtered, t)
			}
		}
	}

	toolNames := make([]string, len(filtered))
	for i, t := range filtered {
		toolNames[i] = t.Name
	}

	inst := &clientInstance{
		cfg:       cfg,
		client:    client,
		tools:     filtered,
		toolNames: toolNames,
		connected: true,
	}

	cm.clients[cfg.Name] = inst
	cm.toolFilter[cfg.Name] = cfg.ToolFilter
	cm.autoConnect[cfg.Name] = cfg.AutoConnect

	cm.logger.Info("mcp client connected",
		slog.String("name", cfg.Name),
		slog.String("transport", cfg.Transport),
		slog.Int("tools", len(filtered)),
	)

	return nil
}

// Disconnect closes a client connection by name.
func (cm *ClientManager) Disconnect(name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.disconnectLocked(name)
}

func (cm *ClientManager) disconnectLocked(name string) error {
	inst, ok := cm.clients[name]
	if !ok {
		return fmt.Errorf("mcp client %q not connected", name)
	}

	if inst.healthTicker != nil {
		inst.healthTicker.Stop()
	}
	if inst.reconnectCh != nil {
		close(inst.reconnectCh)
	}

	if err := inst.client.Close(); err != nil {
		cm.logger.Warn("mcp client close", slog.String("name", name), slog.Any("error", err))
	}

	delete(cm.clients, name)
	delete(cm.toolFilter, name)
	delete(cm.autoConnect, name)

	cm.logger.Info("mcp client disconnected", slog.String("name", name))
	return nil
}

// Get returns a client's tools and state by name.
func (cm *ClientManager) Get(name string) ([]ToolDef, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	inst, ok := cm.clients[name]
	if !ok || !inst.connected {
		return nil, false
	}
	return inst.tools, true
}

// CallTool invokes a tool on a connected MCP client.
func (cm *ClientManager) CallTool(serverName, toolName string, args map[string]any) (map[string]any, error) {
	cm.mu.RLock()
	inst, ok := cm.clients[serverName]
	cm.mu.RUnlock()

	if !ok || !inst.connected {
		return nil, fmt.Errorf("mcp client %q: not connected", serverName)
	}

	// Check tool filter
	if filter, exists := cm.toolFilter[serverName]; exists && len(filter) > 0 {
		if !contains(filter, toolName) {
			return nil, fmt.Errorf("mcp client %q: tool %q not in allowlist", serverName, toolName)
		}
	}

	return inst.client.CallTool(toolName, args)
}

// List returns status snapshots of all connected clients.
func (cm *ClientManager) List() []ClientState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]ClientState, 0, len(cm.clients))
	for _, inst := range cm.clients {
		result = append(result, ClientState{
			Name:      inst.cfg.Name,
			URL:       inst.cfg.URL,
			Transport: inst.cfg.Transport,
			Connected: inst.connected,
			ToolCount: len(inst.tools),
			Tools:     inst.toolNames,
			LastError: inst.lastError,
		})
	}
	return result
}

// AutoConnectAll connects to all configured clients with AutoConnect = true.
// Returns a registry-compatible map of proxy tool names → ToolDef.
func (cm *ClientManager) AutoConnectAll(cfgs []MCPClientServerConfig) []MCPClientServerConfig {
	connected := make([]MCPClientServerConfig, 0)
	for _, cfg := range cfgs {
		if !cfg.AutoConnect {
			continue
		}
		if err := cm.Connect(cfg); err != nil {
			cm.logger.Warn("mcp client auto-connect failed",
				slog.String("name", cfg.Name),
				slog.Any("error", err),
			)
			continue
		}
		connected = append(connected, cfg)
	}
	return connected
}

// StartHealthCheck starts periodic health checks for a client.
// If the server becomes unreachable, it attempts reconnection with backoff.
func (cm *ClientManager) StartHealthCheck(name string, interval time.Duration, maxBackoff time.Duration) {
	cm.mu.RLock()
	inst, ok := cm.clients[name]
	cm.mu.RUnlock()

	if !ok {
		return
	}

	inst.reconnectCh = make(chan struct{})
	inst.healthTicker = time.NewTicker(interval)

	go func() {
		backoff := interval
		for {
			select {
			case <-inst.healthTicker.C:
				cm.mu.RLock()
				_, exists := cm.clients[name]
				connected := exists && cm.clients[name].connected
				cm.mu.RUnlock()

				if !exists {
					return
				}

				if connected {
					// Ping: try listing tools to verify connection
					_, err := inst.client.ListTools()
					if err != nil {
						cm.mu.Lock()
						if cinst, ok := cm.clients[name]; ok {
							cinst.connected = false
							cinst.lastError = err.Error()
						}
						cm.mu.Unlock()

						cm.logger.Warn("mcp client health check failed",
							slog.String("name", name),
							slog.Any("error", err),
						)

						// Attempt reconnect
						go cm.reconnect(inst, backoff, maxBackoff)
					}
				}

			case <-inst.reconnectCh:
				return
			}
		}
	}()
}

func (cm *ClientManager) reconnect(inst *clientInstance, initialBackoff, maxBackoff time.Duration) {
	backoff := initialBackoff
	for {
		select {
		case <-inst.reconnectCh:
			return
		case <-time.After(backoff):
			cm.logger.Info("mcp client reconnecting",
				slog.String("name", inst.cfg.Name),
				slog.Duration("backoff", backoff),
			)

			// Try to reconnect
			if err := inst.client.Close(); err != nil {
				cm.logger.Warn("mcp client close for reconnect",
					slog.String("name", inst.cfg.Name),
					slog.Any("error", err),
				)
			}

			newClient := inst.client
			switch inst.cfg.Transport {
			case "stdio":
				newClient = NewStdioClient(inst.cfg.Name, inst.cfg.Command, inst.cfg.Args, inst.cfg.Env)
			default:
				newClient = NewHTTPClient(inst.cfg.Name, inst.cfg.URL, inst.cfg.Headers)
			}

			if err := newClient.Connect(); err != nil {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			tools, err := newClient.ListTools()
			if err != nil {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				newClient.Close()
				continue
			}

			filtered := tools
			if filter, exists := cm.toolFilter[inst.cfg.Name]; exists && len(filter) > 0 {
				filterSet := make(map[string]bool, len(filter))
				for _, f := range filter {
					filterSet[f] = true
				}
				filtered = make([]ToolDef, 0, len(tools))
				for _, t := range tools {
					if filterSet[t.Name] {
						filtered = append(filtered, t)
					}
				}
			}

			cm.mu.Lock()
			if cinst, ok := cm.clients[inst.cfg.Name]; ok {
				cinst.client = newClient
				cinst.tools = filtered
				cinst.toolNames = make([]string, len(filtered))
				for i, t := range filtered {
					cinst.toolNames[i] = t.Name
				}
				cinst.connected = true
				cinst.lastError = ""
			}
			cm.mu.Unlock()

			cm.logger.Info("mcp client reconnected",
				slog.String("name", inst.cfg.Name),
				slog.Int("tools", len(filtered)),
			)
			return
		}
	}
}

// Shutdown closes all connections.
func (cm *ClientManager) Shutdown() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for name, inst := range cm.clients {
		if inst.healthTicker != nil {
			inst.healthTicker.Stop()
		}
		if inst.reconnectCh != nil {
			close(inst.reconnectCh)
		}
		inst.client.Close()
		delete(cm.clients, name)
	}
	cm.clients = make(map[string]*clientInstance)
	cm.logger.Info("mcp client manager shut down")
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}