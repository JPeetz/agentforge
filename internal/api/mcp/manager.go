// Package mcp — MCP server manager: multi-server lifecycle for HTTP and stdio transports.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/agentforge/agentforge/internal/config"
	"github.com/agentforge/agentforge/internal/mcpclient"
	"github.com/agentforge/agentforge/internal/tool"
)

// ServerState is a public snapshot of a running MCP server's runtime state.
type ServerState struct {
	Name       string   `json:"name"`
	Port       int      `json:"port"`
	Transport  string   `json:"transport"`
	Enabled    bool     `json:"enabled"`
	Running    bool     `json:"running"`
	ToolCount  int      `json:"toolCount"`
	ToolFilter []string `json:"toolFilter,omitempty"`
}

// serverInstance holds the runtime state for a single MCP server.
type serverInstance struct {
	cfg     config.MCPServer
	srv     *Server               // the MCP JSON-RPC endpoint
	httpSrv *http.Server          // set for HTTP transport
	cmd     *exec.Cmd             // set for stdio transport
	stdin   io.WriteCloser        // stdio: command stdin
	stdout  io.ReadCloser         // stdio: command stdout
	mcpHTTP *http.ServeMux        // dedicated mux for HTTP transport
}

// Manager manages all MCP server instances.
type Manager struct {
	servers map[string]*serverInstance // name → instance
	cfg     *config.Config
	reg     *tool.Registry

	mu      sync.RWMutex
	logger  *slog.Logger
}

// NewManager creates an MCP Manager from config and tool registry.
func NewManager(cfg *config.Config, reg *tool.Registry) *Manager {
	return &Manager{
		servers: make(map[string]*serverInstance),
		cfg:     cfg,
		reg:     reg,
		logger:  slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

// SetLogger sets the logger for the manager (for daemon use).
func (m *Manager) SetLogger(l *slog.Logger) { m.logger = l }

// Start initializes and starts all enabled MCP servers from config.
func (m *Manager) Start() error {
	config.SetMCPDefaults(m.cfg)

	for _, srvCfg := range m.cfg.MCP.Servers {
		if !srvCfg.Enabled {
			continue
		}
		if err := m.addServer(srvCfg); err != nil {
			return fmt.Errorf("mcp manager: server %q: %w", srvCfg.Name, err)
		}
	}

	m.logger.Info("mcp manager started",
		slog.Int("servers", len(m.servers)),
	)
	return nil
}

// Stop gracefully shuts down all MCP servers.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, inst := range m.servers {
		m.stopInstance(name, inst)
	}
	m.servers = make(map[string]*serverInstance)
	m.logger.Info("mcp manager stopped")
}

// Add adds and optionally starts a new MCP server.
func (m *Manager) Add(srvCfg config.MCPServer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[srvCfg.Name]; exists {
		return fmt.Errorf("mcp server %q already exists", srvCfg.Name)
	}

	return m.addServerLocked(srvCfg)
}

// Remove stops and removes an MCP server by name.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("mcp server %q not found", name)
	}

	m.stopInstance(name, inst)
	delete(m.servers, name)
	m.logger.Info("mcp server removed", slog.String("name", name))
	return nil
}

// List returns status snapshots of all configured MCP servers.
func (m *Manager) List() []ServerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServerState, 0, len(m.servers))
	for _, inst := range m.servers {
		result = append(result, m.instanceInfo(inst))
	}
	return result
}

// ── Internal helpers ──────────────────────────────────────────────────────

func (m *Manager) addServer(srvCfg config.MCPServer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addServerLocked(srvCfg)
}

func (m *Manager) addServerLocked(srvCfg config.MCPServer) error {
	info := ServerIdentity{
		Name:    "AgentForge",
		Version: m.cfg.Version,
	}

	srv := NewServer(info)

	// Register tools from registry through tool filter
	if m.reg != nil {
		toolNames := m.reg.List()
		for _, tName := range toolNames {
			// Apply tool filter if configured
			if len(srvCfg.ToolFilter) > 0 && !contains(srvCfg.ToolFilter, tName) {
				continue
			}
			t := m.reg.Get(tName)
			if t == nil {
				continue
			}
			meta := t.Meta()
			mcpTool := Tool{
				Name:        meta.Name,
				Description: meta.Description,
				InputSchema: meta.InputSchema,
			}
			// Capture tool reference for handler
			toolRef := t
			srv.RegisterTool(mcpTool, func(ctx context.Context, args map[string]any) (string, error) {
				result, err := toolRef.Execute(ctx, args)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(result)
				return string(b), nil
			})
		}
	}

	inst := &serverInstance{
		cfg: srvCfg,
		srv: srv,
	}

	// Set up transport
	switch srvCfg.Transport {
	case "stdio":
		if err := m.startStdioLocked(inst); err != nil {
			return err
		}
	default:
		if srvCfg.Transport == "" {
			srvCfg.Transport = "http"
		}
		if err := m.startHTTPLocked(inst); err != nil {
			return err
		}
	}

	m.servers[srvCfg.Name] = inst
	m.logger.Info("mcp server added",
		slog.String("name", srvCfg.Name),
		slog.String("transport", srvCfg.Transport),
		slog.Int("port", srvCfg.Port),
		slog.Int("tools", len(srv.tools)),
	)

	return nil
}

func (m *Manager) startHTTPLocked(inst *serverInstance) error {
	mux := http.NewServeMux()
	mux.Handle("/", inst.srv)
	inst.mcpHTTP = mux

	addr := fmt.Sprintf(":%d", inst.cfg.Port)
	inst.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		m.logger.Info("mcp http server listening",
			slog.String("name", inst.cfg.Name),
			slog.String("addr", addr),
		)
		if err := inst.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("mcp http server",
				slog.String("name", inst.cfg.Name),
				slog.Any("error", err),
			)
		}
	}()

	return nil
}

func (m *Manager) startStdioLocked(inst *serverInstance) error {
	if inst.cfg.Command == "" {
		return fmt.Errorf("stdio transport requires a command")
	}

	cmd := exec.Command(inst.cfg.Command, inst.cfg.Args...)

	// Use filtered environment (blocks secrets like API keys, tokens, passwords)
	cmd.Env = mcpclient.FilterEnvironment(inst.cfg.Env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdio stdin pipe: %w", err)
	}
	inst.stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdio stdout pipe: %w", err)
	}
	inst.stdout = stdout

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("stdio start command: %w", err)
	}
	inst.cmd = cmd

	// Start the read loop that feeds JSON-RPC via stdout
	go func() {
		dec := json.NewDecoder(stdout)
		enc := json.NewEncoder(stdin)

		for {
			var req Request
			if err := dec.Decode(&req); err != nil {
				if err == io.EOF {
					return
				}
				m.logger.Error("mcp stdio decode",
					slog.String("name", inst.cfg.Name),
					slog.Any("error", err),
				)
				inst.srv.writeError(enc, nil, -32700, "Parse error", err.Error())
				continue
			}

			resp := inst.srv.handleRequest(context.Background(), req)
			if err := enc.Encode(resp); err != nil {
				m.logger.Error("mcp stdio encode",
					slog.String("name", inst.cfg.Name),
					slog.Any("error", err),
				)
				return
			}
		}
	}()

	return nil
}

func (m *Manager) stopInstance(name string, inst *serverInstance) {
	if inst.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := inst.httpSrv.Shutdown(ctx); err != nil {
			m.logger.Warn("mcp http shutdown",
				slog.String("name", name),
				slog.Any("error", err),
			)
		}
	}

	if inst.stdin != nil {
		inst.stdin.Close()
	}
	if inst.cmd != nil && inst.cmd.Process != nil {
		inst.cmd.Process.Kill()
		inst.cmd.Wait()
	}
}

func (m *Manager) instanceInfo(inst *serverInstance) ServerState {
	toolCount := 0
	if inst.srv != nil {
		inst.srv.mu.RLock()
		toolCount = len(inst.srv.tools)
		inst.srv.mu.RUnlock()
	}

	running := false
	if inst.httpSrv != nil || (inst.cmd != nil && inst.cmd.Process != nil) {
		running = true
	}

	return ServerState{
		Name:       inst.cfg.Name,
		Port:       inst.cfg.Port,
		Transport:  inst.cfg.Transport,
		Enabled:    inst.cfg.Enabled,
		Running:    running,
		ToolCount:  toolCount,
		ToolFilter: inst.cfg.ToolFilter,
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}