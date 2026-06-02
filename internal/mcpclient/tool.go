// Package mcpclient — MCP proxy tool that wraps external MCP tools
// as AgentForge-native tools for the tool registry.
package mcpclient

import (
	"context"
	"fmt"

	"github.com/agentforge/agentforge/internal/tool"
)

// ToolPrefix is prepended to all MCP proxy tool names to prevent
// collisions with built-in and WASM tools.
const ToolPrefix = "mcp_"

// MCPProxyTool wraps an external MCP server tool and makes it available
// as an AgentForge tool.Tool in the tool registry.
//
// Tool names are constructed as: mcp_{serverName}_{toolName}
// e.g. "mcp_skills-server_seo-audit"
type MCPProxyTool struct {
	serverName string
	toolDef    ToolDef
	manager    *ClientManager
}

// NewMCPProxyTool creates a proxy tool for an external MCP tool.
func NewMCPProxyTool(serverName string, td ToolDef, mgr *ClientManager) *MCPProxyTool {
	return &MCPProxyTool{
		serverName: serverName,
		toolDef:    td,
		manager:    mgr,
	}
}

// BuildProxyName constructs the registry-safe tool name for an MCP proxy tool.
func BuildProxyName(serverName, toolName string) string {
	return fmt.Sprintf("%s%s_%s", ToolPrefix, serverName, toolName)
}

// Meta returns the tool metadata with the proxied schema and description.
func (p *MCPProxyTool) Meta() tool.Metadata {
	return tool.Metadata{
		Name:        BuildProxyName(p.serverName, p.toolDef.Name),
		Description: fmt.Sprintf("[MCP:%s] %s", p.serverName, p.toolDef.Description),
		Version:     "mcp-proxy",
		Author:      "AgentForge MCP Client",
		Category:    "mcp",
		InputSchema: p.toolDef.InputSchema,
	}
}

// Execute forwards the call to the MCP client manager's CallTool.
func (p *MCPProxyTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	result, err := p.manager.CallTool(p.serverName, p.toolDef.Name, args)
	if err != nil {
		return nil, fmt.Errorf("mcp proxy [%s/%s]: %w", p.serverName, p.toolDef.Name, err)
	}
	return result, nil
}

// Ensure interface compliance at compile time.
var _ tool.Tool = (*MCPProxyTool)(nil)