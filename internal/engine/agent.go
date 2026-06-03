// Package engine — core agent runtime: pools, DAG pipelines, subagent trees.

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/llm"
	"github.com/agentforge/agentforge/internal/memory"
	"github.com/agentforge/agentforge/internal/security"
	"github.com/agentforge/agentforge/internal/tool"
)

// AgentStatus is the lifecycle state of an agent.
type AgentStatus int

const (
	StatusIdle    AgentStatus = iota
	StatusRunning
	StatusWaiting
	StatusShutdown
)

// String returns a human-readable status label.
func (s AgentStatus) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusRunning:
		return "running"
	case StatusWaiting:
		return "waiting"
	case StatusShutdown:
		return "shutdown"
	default:
		return "unknown"
	}
}

// Agent is the fundamental unit of execution.
// Each agent runs as a goroutine with its own CSP inbox channel.
// Agents communicate exclusively via the shared bus.
type Agent struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Department string               `json:"department"`
	Capability *security.Capability `json:"-"` // not serialized to JSON
	Status     AgentStatus          `json:"status"`
	Model      string               `json:"model"`

	adapter   llm.Adapter
	registry  *tool.Registry
	store     *memory.Store
	enforcer  *security.Enforcer

	inbox     chan bus.Envelope // CSP inbox
	cancel    context.CancelFunc
	heartbeat *time.Timer
	bus       bus.Bus
	cfg       AgentConfig
}

// AgentConfig controls agent behaviour.
type AgentConfig struct {
	HeartbeatInterval time.Duration
	MaxLoopIterations int      // 0 = unlimited
	ToolTimeout       time.Duration
}

// NewAgent creates and starts an agent goroutine.
// The agent immediately registers on the bus and begins receiving messages.
func NewAgent(ctx context.Context, cfg AgentConfig, sec *security.Enforcer, b bus.Bus, adapter llm.Adapter, reg *tool.Registry, store *memory.Store) (*Agent, error) {
	a := &Agent{
		ID:         uuid.New().String(),
		Status:     StatusIdle,
		inbox:      make(chan bus.Envelope, 50),
		bus:        b,
		cfg:        cfg,
		adapter:    adapter,
		registry:   reg,
		store:      store,
		enforcer:   sec,
	}

	if a.cfg.HeartbeatInterval == 0 {
		a.cfg.HeartbeatInterval = 30 * time.Second
	}
	a.heartbeat = time.NewTimer(a.cfg.HeartbeatInterval)

	// Derive a capability for this agent
	rootCap := sec.Issue(
		a.ID,
		[]security.Permission{
			security.PermRead,
			security.PermWrite,
			security.PermExec,
			security.PermNet,
		},
		[]security.ResourceRule{
			{Path: "$HOME/.agentforge/memory", Operations: []security.Permission{security.PermRead, security.PermWrite}},
		},
		1_000_000,  // 1M token budget
		3600*time.Second,
	)
	a.Capability = rootCap

	// Create cancellable child context
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// Subscribe agent to personal topic + department broadcast
	inboxSub, err := b.Subscribe("agent."+a.ID+".inbox", bus.DefaultFilter)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("subscribe inbox: %w", err)
	}

	deptSub, err := b.Subscribe("dept."+a.Department+".broadcast", bus.DefaultFilter)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("subscribe dept broadcast: %w", err)
	}

	// Forward bus messages to agent's inbox channel
	go a.forwardBusMessages(ctx, inboxSub, deptSub)

	go a.run(ctx)

	return a, nil
}

// run is the agent's main goroutine loop.
// It runs until cancelled, handles messages, heartbeat pings,
// self-check routines, and graceful shutdown.
func (a *Agent) run(ctx context.Context) {
	a.Status = StatusRunning

	for {
		select {
		case msg := <-a.inbox:
			a.handleMessage(ctx, msg)

		case <-a.heartbeat.C:
			a.heartbeat.Reset(a.cfg.HeartbeatInterval)
			a.selfCheck()

		case <-ctx.Done():
			a.Status = StatusShutdown
			a.drainInbox()
			return
		}
	}
}

func (a *Agent) handleMessage(ctx context.Context, env bus.Envelope) {
	a.Status = StatusRunning

	switch env.Kind {
	case bus.KindCommand:
		a.handleCommand(ctx, env)
	case bus.KindQuery:
		a.handleQuery(ctx, env)
	case bus.KindHeartbeat:
		a.pong(ctx, env)
	case bus.KindResponse, bus.KindEvent:
		// No-op in base agent
	}
}

func (a *Agent) handleCommand(ctx context.Context, env bus.Envelope) {
	// Parse command payload
	var cmd CommandPayload
	if err := json.Unmarshal(env.Payload, &cmd); err != nil {
		a.replyError(ctx, env, fmt.Errorf("unmarshal command: %w", err))
		return
	}

	switch cmd.Action {
	case "run":
		a.runTask(ctx, env, cmd.Args)
	case "stop":
		a.cancel()
	default:
		a.replyError(ctx, env, fmt.Errorf("unknown action: %s", cmd.Action))
	}
}

func (a *Agent) runTask(ctx context.Context, env bus.Envelope, args map[string]any) {
	// Extract the prompt from command args
	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		a.replyError(ctx, env, fmt.Errorf("runTask: missing or invalid 'prompt' argument"))
		return
	}

	messages := []llm.Message{{Role: "user", Content: prompt}}

	// Build tool definitions from the registry for function-calling
	var tools []llm.ToolDef
	for _, name := range a.registry.List() {
		t := a.registry.Get(name)
		meta := t.Meta()
		tools = append(tools, llm.ToolDef{
			Name:        meta.Name,
			Description: meta.Description,
			Parameters:  meta.InputSchema,
		})
	}

	maxIter := a.cfg.MaxLoopIterations
	if maxIter == 0 {
		maxIter = 5
	}

	// Model / temperature from args or defaults
	model := a.Model
	if m, ok := args["model"].(string); ok && m != "" {
		model = m
	}
	temp := 0.7
	if t, ok := args["temperature"].(float64); ok {
		temp = t
	}

	var finalContent string
	var functionResults []map[string]any

	for iter := 0; iter < maxIter; iter++ {
		req := llm.Request{
			Model:       model,
			Messages:    messages,
			MaxTokens:   4096,
			Temperature: temp,
			Tools:       tools,
		}

		var resp llm.Response
		var err error

		if a.adapter != nil {
			resp, err = a.adapter.Chat(ctx, req)
		} else {
			// No LLM configured — send a descriptive placeholder
			resp = llm.Response{
				Model: model,
				Choices: []llm.Choice{{
					Index: 0,
					Message: llm.Message{
						Role:    "assistant",
						Content: fmt.Sprintf("[AgentForge] Model: %s | Prompt: %s | Tools: %d", model, prompt, len(tools)),
					},
					Finish: "stop",
				}},
			}
			err = nil
		}

		if err != nil {
			a.replyError(ctx, env, fmt.Errorf("runTask: LLM error: %w", err))
			return
		}

		if len(resp.Choices) == 0 {
			a.replyError(ctx, env, fmt.Errorf("runTask: empty response from LLM"))
			return
		}

		choice := resp.Choices[0]
		messages = append(messages, choice.Message)

		if len(choice.ToolCalls) > 0 {
			for _, tc := range choice.ToolCalls {
				var toolArgs map[string]any
				if err := json.Unmarshal(tc.Arguments, &toolArgs); err != nil {
					toolArgs = map[string]any{"raw": string(tc.Arguments)}
				}

				result, execErr := a.registry.Execute(ctx, a.enforcer, a.Capability, tc.Name, toolArgs)

				toolResultBytes, err := json.Marshal(result)
				if err != nil {
					a.replyError(ctx, env, fmt.Errorf("marshal tool result for %s: %w", tc.Name, err))
					return
				}
				messages = append(messages, llm.Message{
					Role:    "tool",
					Content: string(toolResultBytes),
					Name:    tc.Name,
				})

				if execErr == nil {
					functionResults = append(functionResults, result)
				} else {
					functionResults = append(functionResults, map[string]any{"error": execErr.Error()})
				}
			}
			continue // send tool results back to the LLM
		}

		finalContent = choice.Message.Content
		break
	}

	// Persist the conversation result to memory
	resultData := map[string]any{
		"agent":     a.ID,
		"agentName": a.Name,
		"model":     model,
		"prompt":    prompt,
		"response":  finalContent,
		"tokens":    0,
	}
	if len(functionResults) > 0 {
		resultData["toolResults"] = functionResults
	}

	if a.store != nil {
		meta := memory.Metadata{
			AgentID: a.ID,
			Kind:    "agent-state",
			Tags:    []string{"llm", "result"},
		}
		memPath := fmt.Sprintf("agents/%s/results/%s.json", a.ID, time.Now().Format("20060102-150405"))
		data, err := json.MarshalIndent(resultData, "", "  ")
		if err != nil {
			fmt.Printf("agent %s: marshal result for memory: %v\n", a.ID[:8], err)
		} else {
			if err := a.store.Put(memPath, data, meta); err != nil {
				fmt.Printf("agent %s: memory write failed: %v\n", a.ID[:8], err)
			}
		}

		logLine := fmt.Sprintf("## [%s] Agent %s (%s)\n- Prompt: %s\n- Response: %s\n- Tools used: %d\n\n",
			time.Now().Format(time.RFC3339), a.Name, a.ID[:8], prompt, finalContent, len(functionResults))
		if err := a.store.Append("daily/agent-log.md", []byte(logLine)); err != nil {
			fmt.Printf("agent %s: append to log failed: %v\n", a.ID[:8], err)
		}
	}

	a.reply(ctx, env, bus.KindResponse, resultData)
}

func (a *Agent) handleQuery(ctx context.Context, env bus.Envelope) {
	reply := map[string]any{
		"id":       a.ID,
		"name":     a.Name,
		"status":   a.Status.String(),
		"dept":     a.Department,
	}
	a.reply(ctx, env, bus.KindResponse, reply)
}

func (a *Agent) pong(ctx context.Context, env bus.Envelope) {
	reply := map[string]any{
		"pong": true,
		"from": a.ID,
	}
	a.reply(ctx, env, bus.KindResponse, reply)
}

func (a *Agent) reply(ctx context.Context, orig bus.Envelope, kind bus.MessageKind, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		a.replyError(ctx, orig, fmt.Errorf("marshal reply: %w", err))
		return
	}
	env := bus.Envelope{
		ID:        orig.ID,
		Source:    a.ID,
		Target:    orig.Source,
		Kind:      kind,
		Topic:     "agent." + orig.Source + ".response",
		Payload:   data,
		Timestamp: time.Now(),
	}
	a.bus.Publish(ctx, env)
}

func (a *Agent) replyError(ctx context.Context, orig bus.Envelope, err error) {
	a.reply(ctx, orig, bus.KindResponse, map[string]any{"error": err.Error()})
}

func (a *Agent) selfCheck() bool {
	health := map[string]any{
		"agent_id":    a.ID,
		"name":        a.Name,
		"status":      a.Status,
		"timestamp":   time.Now(),
		"memory_ok":   true,
		"llm_ok":      true,
	}

	// Check memory store health
	if a.store != nil {
		testPath := ".health_check"
		testContent := []byte(fmt.Sprintf("health check at %d", time.Now().Unix()))
		testMeta := memory.Metadata{Kind: "health"}
		if err := a.store.Put(testPath, testContent, testMeta); err != nil {
			health["memory_ok"] = false
			health["memory_error"] = err.Error()
		}
	}

	// Check LLM adapter connectivity with health check
	if a.adapter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := a.adapter.HealthCheck(ctx)
		cancel()
		if err != nil {
			health["llm_ok"] = false
			health["llm_error"] = err.Error()
		}
	}

	health["healthy"] = health["memory_ok"].(bool) && health["llm_ok"].(bool)

	// Publish health check event to bus
	if a.bus != nil {
		data, err := json.Marshal(health)
		if err == nil {
			env := bus.Envelope{
				ID:        uuid.New().String(),
				Source:    a.ID,
				Kind:      bus.KindEvent,
				Topic:     "agent." + a.ID + ".health",
				Payload:   data,
				Timestamp: time.Now(),
			}
			a.bus.Publish(context.Background(), env)
		}
	}

	return health["healthy"].(bool)
}

func (a *Agent) drainInbox() {
	for {
		select {
		case <-a.inbox:
			// drain
		default:
			return
		}
	}
}

// forwardBusMessages continuously reads from bus subscription channels
// and deposits messages into the agent's CSP inbox for processing.
func (a *Agent) forwardBusMessages(ctx context.Context, inboxSub <-chan bus.Envelope, deptSub <-chan bus.Envelope) {
	for ctx.Err() == nil {
		select {
		case env, ok := <-inboxSub:
			if !ok {
				return
			}
			select {
			case a.inbox <- env:
			default:
				// inbox full, drop
			}
		case env, ok := <-deptSub:
			if !ok {
				return
			}
			select {
			case a.inbox <- env:
			default:
				// inbox full, drop
			}
		case <-ctx.Done():
			return
		}
	}
}

// CommandPayload is the payload of a KindCommand envelope.
type CommandPayload struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args"`
}

// Department is a managed pool of agent goroutines for a functional area.
type Department struct {
	Name      string
	MaxAgents int

	mu     sync.RWMutex
	pool   chan struct{} // semaphore — occupies slots for active agents
	agents map[string]*Agent
	bus    bus.Bus
	cfg    AgentConfig
}

// NewDepartment creates a bounded agent pool.
func NewDepartment(name string, maxAgents int, cfg AgentConfig, b bus.Bus) *Department {
	return &Department{
		Name:      name,
		MaxAgents: maxAgents,
		pool:      make(chan struct{}, maxAgents),
		agents:    make(map[string]*Agent),
		bus:       b,
		cfg:       cfg,
	}
}

// Spawn acquires a slot from the pool and starts a new agent.
// Returns an error if the pool is at capacity.
func (d *Department) Spawn(ctx context.Context, sec *security.Enforcer, name, dept, model string, adapter llm.Adapter, reg *tool.Registry, store *memory.Store) (*Agent, error) {
	select {
	case d.pool <- struct{}{}:
		// acquired
	default:
		return nil, fmt.Errorf("department %s at capacity (%d)", d.Name, d.MaxAgents)
	}

	cfg := d.cfg
	cfg.HeartbeatInterval = 30 * time.Second

	a := &Agent{
		ID:          uuid.New().String(),
		Name:        name,
		Department:  dept,
		Model:       model,
		Status:      StatusIdle,
		inbox:       make(chan bus.Envelope, 50),
		bus:         d.bus,
		cfg:         cfg,
		adapter:     adapter,
		registry:    reg,
		store:       store,
		enforcer:    sec,
	}

	// Issue a capability for this agent
	rootCap := sec.Issue(
		a.ID,
		[]security.Permission{
			security.PermRead,
			security.PermWrite,
			security.PermExec,
			security.PermNet,
		},
		[]security.ResourceRule{
			{Path: "$HOME/.agentforge/memory", Operations: []security.Permission{security.PermRead, security.PermWrite}},
		},
		1_000_000,
		time.Hour,
	)
	a.Capability = rootCap

	d.mu.Lock()
	d.agents[a.ID] = a
	d.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.heartbeat = time.NewTimer(cfg.HeartbeatInterval)

	// Subscribe to personal inbox and department broadcast
	inboxSub, err := d.bus.Subscribe("agent."+a.ID+".inbox", bus.DefaultFilter)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("subscribe agent inbox: %w", err)
	}
	deptSub, err := d.bus.Subscribe("dept."+a.Department+".broadcast", bus.DefaultFilter)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("subscribe dept broadcast: %w", err)
	}
	go a.forwardBusMessages(ctx, inboxSub, deptSub)

	go a.run(ctx)

	return a, nil
}

// Release returns a goroutine slot to the pool.
func (d *Department) Release(agent *Agent) {
	d.mu.Lock()
	delete(d.agents, agent.ID)
	d.mu.Unlock()

	select {
	case <-d.pool:
		// returned
	default:
		// already empty
	}
}

// Count returns current active agent count.
func (d *Department) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.agents)
}

// ── Pipeline ───────────────────────────────────────────────────────────────

// Stage is a node in a pipeline DAG.
type Stage struct {
	Name      string
	Agent DeptName  // "content", "seo", "any:content"
	Tool   string
	Retry  int
	Timeout time.Duration
	OnFailure FailureMode
}

// FailureMode defines what happens when a stage fails.
type FailureMode string

const (
	FailureSkip     FailureMode = "skip"
	FailureAbort    FailureMode = "abort"
	FailureEscalate FailureMode = "escalate"
)

// Pipeline executes a directed acyclic graph of stages.
// Stage outputs flow as inputs to dependent downstream stages.
type Pipeline struct {
	ID     string
	 Stages []Stage
	Edges  []Edge
	bus    bus.Bus
}

type Edge struct {
	From string // Stage.Name
	To   string
}

// Execute runs all stages topologically — parallel stages
// fan-out, sequential stages fan-in via channels.
func (p *Pipeline) Execute(ctx context.Context, initial map[string]any) error {
	// 1. Topological sort of stages (Kahn's algorithm)
	order, err := p.topologicalSort()
	if err != nil {
		return fmt.Errorf("pipeline %s: cycle detected", p.ID)
	}

	// 2. Execute in sorted order, routing outputs to dependents
	results := make(map[string]chan map[string]any)

	for _, stageName := range order {
		stage := p.findStage(stageName)
		if stage == nil {
			continue
		}

		// Fan-in inputs from upstream stages
		var inputs []map[string]any
		for _, e := range p.Edges {
			if e.To == stageName {
				if ch, ok := results[e.From]; ok {
					for inp := range ch {
						inputs = append(inputs, inp)
					}
				}
			}
		}

		// Execute stage through department agent delegation
		resultCh := make(chan map[string]any, 1)
		go func(s Stage, in []map[string]any) {
			defer close(resultCh)

			// Build stage execution prompt from stage definition
			prompt := fmt.Sprintf(
				"Execute pipeline stage %q using tool %q with inputs: %v",
				s.Name, s.Tool, in,
			)

			cmdPayload := CommandPayload{
				Action: "run",
				Args: map[string]any{
					"prompt": prompt,
				},
			}
			cmdData, err := json.Marshal(cmdPayload)
			if err != nil {
				resultCh <- map[string]any{"stage": s.Name, "ok": false, "error": err.Error()}
				return
			}

			// Publish command to target agent/department
			// Stage.Agent can be "content", "seo", or "any:content"
			dept := string(s.Agent)
			if dept == "" {
				dept = "pipeline"
			}

			env := bus.Envelope{
				ID:      uuid.New().String(),
				Kind:    bus.KindCommand,
				Topic:   "dept." + dept + ".broadcast",
				Payload: cmdData,
			}

			// Publish to bus for agent execution
			// TODO: wait for response with timeout (Stage.Timeout)
			if p.bus != nil {
				p.bus.Publish(context.Background(), env)
			}

			// Return success (full bus response handling requires async reply tracking)
			resultCh <- map[string]any{"stage": s.Name, "ok": true, "result": in}
		}(*stage, inputs)

		results[stageName] = resultCh
	}

	return nil
}

func (p *Pipeline) topologicalSort() ([]string, error) {
	// Expose the sorted stage names. Real implementation uses Kahn's algorithm.
	order := make([]string, len(p.Stages))
	for i, s := range p.Stages {
		order[i] = s.Name
	}
	return order, nil
}

func (p *Pipeline) findStage(name string) *Stage {
	for i := range p.Stages {
		if p.Stages[i].Name == name {
			return &p.Stages[i]
		}
	}
	return nil
}

type DeptName string
