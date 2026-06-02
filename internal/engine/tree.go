// Package tree — subagent tree manager with capability delegation.
//
// A subagent tree is a hierarchical structure where a parent agent
// spawns child agents, each with a derived capability (strict subset
// of the parent's permissions). Results aggregate upward.
//
// Constraints:
//   - Max depth: 5 levels (configurable)
//   - Child capabilities MUST be a subset of parent capabilities
//   - Timeout cascades: parent timeout → all children terminate
//   - Results aggregate upward through channels
package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/security"
)

// ── Tree Manager ─────────────────────────────────────────────────────────────

// TreeConfig controls tree behaviour.
type TreeConfig struct {
	MaxDepth      int           // maximum tree depth (default: 5)
	DefaultTimeout time.Duration // default timeout for subagent operations
	HeartbeatInterval time.Duration
}

// DefaultTreeConfig returns sensible defaults.
func DefaultTreeConfig() TreeConfig {
	return TreeConfig{
		MaxDepth:          5,
		DefaultTimeout:    5 * time.Minute,
		HeartbeatInterval: 10 * time.Second,
	}
}

// TreeManager manages the lifecycle of agent trees.
// It enforces capability delegation (child ⊂ parent) and depth limits.
type TreeManager struct {
	cfg TreeConfig
	bus bus.Bus
	sec *security.Enforcer

	mu    sync.RWMutex
	trees map[string]*TreeNode // root agent ID → tree
}

// NewTreeManager creates a tree manager.
func NewTreeManager(cfg TreeConfig, b bus.Bus, sec *security.Enforcer) *TreeManager {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 5
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = 5 * time.Minute
	}
	return &TreeManager{
		cfg:   cfg,
		bus:   b,
		sec:   sec,
		trees: make(map[string]*TreeNode),
	}
}

// TreeNode is a node in the agent tree.
type TreeNode struct {
	Agent      *Agent
	Parent     *TreeNode
	Children   []*TreeNode
	Depth      int
	Capability *security.Capability // derived from parent, ⊂ parent.cap

	mu       sync.RWMutex
	status   NodeStatus
	result   chan TreeResult // aggregates child results
	cancel   context.CancelFunc
}

// NodeStatus is the lifecycle state of a tree node.
type NodeStatus int

const (
	NodeIdle    NodeStatus = iota
	NodeRunning
	NodeWaiting // waiting for children
	NodeDone
	NodeFailed
)

// TreeResult holds the outcome of a subagent operation.
type TreeResult struct {
	NodeID  string
	AgentID string
	Output  map[string]any
	Error   error
	Depth   int
}

// ── Root Operations ──────────────────────────────────────────────────────────

// SpawnRoot creates a new tree rooted at the given agent.
// This agent becomes the root (depth 0). All child agents will be
// spawned by this agent with derived capabilities.
func (tm *TreeManager) SpawnRoot(ctx context.Context, agent *Agent) (*TreeNode, error) {
	if agent == nil || agent.Capability == nil {
		return nil, fmt.Errorf("tree: agent must have a capability token")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	root := &TreeNode{
		Agent:      agent,
		Depth:      0,
		Capability: agent.Capability,
		status:     NodeRunning,
		result:     make(chan TreeResult, 10),
	}

	tm.trees[agent.ID] = root
	return root, nil
}

// SpawnChild creates a child node under a parent.
// The child's capability is derived from the parent's — it MUST be
// a strict subset (or equal set) of the parent's permissions.
func (tm *TreeManager) SpawnChild(ctx context.Context, parent *TreeNode, agent *Agent, restrictions ...security.Restriction) (*TreeNode, error) {
	parent.mu.RLock()
	depth := parent.Depth + 1
	parent.mu.RUnlock()

	if depth > tm.cfg.MaxDepth {
		return nil, fmt.Errorf("tree: max depth %d exceeded at depth %d", tm.cfg.MaxDepth, depth)
	}

	// Derive capability from parent (subset)
	childCap, err := tm.sec.Derive(parent.Capability, agent.ID, restrictions...)
	if err != nil {
		return nil, fmt.Errorf("tree: derive capability: %w", err)
	}

	node := &TreeNode{
		Agent:      agent,
		Parent:     parent,
		Depth:      depth,
		Capability: childCap,
		status:     NodeRunning,
		result:     make(chan TreeResult, 5),
	}

	// Update the agent's capability to the derived one
	agent.Capability = childCap

	parent.mu.Lock()
	parent.Children = append(parent.Children, node)
	parent.mu.Unlock()

	return node, nil
}

// ── Execution ────────────────────────────────────────────────────────────────

// AwaitAll waits for all children of node to report a result (or timeout).
// Results are collected into a single aggregated map.
func (tm *TreeManager) AwaitAll(ctx context.Context, node *TreeNode, timeout time.Duration) ([]TreeResult, error) {
	if timeout == 0 {
		timeout = tm.cfg.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	node.mu.RLock()
	n := len(node.Children)
	node.mu.RUnlock()

	if n == 0 {
		return nil, nil
	}

	results := make([]TreeResult, 0, n)
	deadline := time.After(timeout)

	for i := 0; i < n; i++ {
		select {
		case r := <-node.result:
			results = append(results, r)
		case <-deadline:
			return results, fmt.Errorf("tree: await timed out after %s (%d/%d children reported)", timeout, len(results), n)
		case <-ctx.Done():
			return results, ctx.Err()
		}
	}

	return results, nil
}

// CollectAll recursively collects results from all descendants.
func (tm *TreeManager) CollectAll(node *TreeNode) []TreeResult {
	var results []TreeResult

	node.mu.RLock()
	for _, child := range node.Children {
		child.mu.RLock()
		if child.status == NodeDone || child.status == NodeFailed {
			select {
			case r := <-child.result:
				results = append(results, r)
			default:
			}
		}
		child.mu.RUnlock()
		// Recurse into grandchildren
		results = append(results, tm.CollectAll(child)...)
	}
	node.mu.RUnlock()

	return results
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

// ReportResult sends a result from this node to its parent.
func (tm *TreeManager) ReportResult(node *TreeNode, output map[string]any, err error) {
	r := TreeResult{
		NodeID:  node.Agent.ID,
		AgentID: node.Agent.ID,
		Output:  output,
		Error:   err,
		Depth:   node.Depth,
	}

	node.mu.Lock()
	if err != nil {
		node.status = NodeFailed
	} else {
		node.status = NodeDone
	}
	node.mu.Unlock()

	if node.Parent != nil {
		select {
		case node.Parent.result <- r:
		default:
			// parent's result buffer full — drop (parent will poll)
		}
	}
}

// CancelTree terminates all agents in a tree, starting from the root.
// Children are cancelled first (bottom-up).
func (tm *TreeManager) CancelTree(root *TreeNode) {
	var cancelAll func(node *TreeNode)
	cancelAll = func(node *TreeNode) {
		node.mu.RLock()
		for _, child := range node.Children {
			cancelAll(child)
		}
		node.mu.RUnlock()
		if node.Agent.cancel != nil {
			node.Agent.cancel()
		}
	}
	cancelAll(root)
}

// ── Depth-First Parallel Execution ──────────────────────────────────────────

// ExecuteTree runs a tree in depth-first parallel mode:
//   - Each node's children execute in parallel (fan-out)
//   - Parent waits for all children (fan-in)
//   - Results bubble up to root
func (tm *TreeManager) ExecuteTree(ctx context.Context, root *TreeNode, timeout time.Duration) ([]TreeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return tm.executeNode(ctx, root)
}

func (tm *TreeManager) executeNode(ctx context.Context, node *TreeNode) ([]TreeResult, error) {
	node.mu.RLock()
	children := make([]*TreeNode, len(node.Children))
	copy(children, node.Children)
	node.mu.RUnlock()

	if len(children) == 0 {
		return nil, nil
	}

	// Fan-out: execute all children in parallel
	var wg sync.WaitGroup
	var allResults []TreeResult
	var errs []error
	var mu sync.Mutex

	for _, child := range children {
		wg.Add(1)
		go func(c *TreeNode) {
			defer wg.Done()

			// Recursively execute this child's subtree
			grandResults, err := tm.executeNode(ctx, c)
			mu.Lock()
			if err != nil {
				errs = append(errs, err)
			}
			allResults = append(allResults, grandResults...)
			mu.Unlock()

			// Report child result
			r := TreeResult{
				NodeID:  c.Agent.ID,
				AgentID: c.Agent.ID,
				Depth:   c.Depth,
			}
			c.mu.RLock()
			if c.status == NodeFailed {
				r.Error = fmt.Errorf("child agent %s failed", c.Agent.ID)
			}
			c.mu.RUnlock()
			mu.Lock()
			allResults = append(allResults, r)
			mu.Unlock()
		}(child)
	}

	wg.Wait()

	if len(errs) > 0 {
		return allResults, fmt.Errorf("tree: %d child errors", len(errs))
	}

	return allResults, nil
}

// ── Tree Status ─────────────────────────────────────────────────────────────

// Status returns a summary of the tree state.
func (tm *TreeManager) Status(rootID string) (map[string]any, error) {
	tm.mu.RLock()
	root, ok := tm.trees[rootID]
	tm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tree: root %s not found", rootID)
	}

	var countNodes func(*TreeNode) int
	countNodes = func(n *TreeNode) int {
		c := 1
		n.mu.RLock()
		for _, child := range n.Children {
			c += countNodes(child)
		}
		n.mu.RUnlock()
		return c
	}

	return map[string]any{
		"root_id":    rootID,
		"depth":      root.Depth,
		"total_nodes": countNodes(root),
		"status":     root.statusString(),
	}, nil
}

func (n *TreeNode) statusString() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	switch n.status {
	case NodeIdle:
		return "idle"
	case NodeRunning:
		return "running"
	case NodeWaiting:
		return "waiting"
	case NodeDone:
		return "done"
	case NodeFailed:
		return "failed"
	default:
		return "unknown"
	}
}


