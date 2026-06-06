// Session management: compaction, pruning, transcripts
// Tracks session history, auto-compacts when nearing context limits, prunes tool outputs.
package session

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ── Types ────────────────────────────────────────────────────────────────────

// Turn is a single message turn in a session transcript.
type Turn struct {
	Timestamp time.Time `json:"ts"`
	Role      string    `json:"role"`            // user, assistant, system, tool
	Content   string    `json:"content,omitempty"`
	ToolName  string    `json:"tool,omitempty"`  // for tool-result turns
	ToolID    string    `json:"tool_id,omitempty"`
	Tokens    int       `json:"tokens,omitempty"` // estimated token count
}

// Session holds one conversation session.
type Session struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Turns     []Turn    `json:"turns"`
	Compacted int       `json:"compacted_count"` // number of times compacted
	mu        sync.RWMutex
}

// Summary is a compaction result placed at the front of the transcript.
type Summary struct {
	Summary   string    `json:"summary"`
	FromTurns int       `json:"from_turns"` // how many turns summarized
	At        time.Time `json:"at"`
}

// Config controls compaction & pruning behavior.
type Config struct {
	MaxContextTokens   int  `yaml:"maxContextTokens"`   // e.g. 128000
	AutoCompactAt      int  `yaml:"autoCompactAt"`      // % of max, e.g. 80
	KeepRecentTokens   int  `yaml:"keepRecentTokens"`   // token tail to keep unsummarized
	NotifyUser         bool `yaml:"notifyUser"`
	TruncateAfter      bool `yaml:"truncateAfterCompaction"`
	PruneToolOutputs   bool `yaml:"pruneToolOutputs"`
	MaxToolOutputChars int  `yaml:"maxToolOutputChars"` // trim tool output beyond this
	// Compaction model override — if empty, uses the agent's primary LLM
	ModelOverride string `yaml:"modelOverride,omitempty"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxContextTokens:   0,       // 0 = auto-detect from LLM adapter
		AutoCompactAt:      90,      // 90% of context window
		KeepRecentTokens:   8000,    // 8K tail preserved
		NotifyUser:         true,
		TruncateAfter:      true,
		PruneToolOutputs:   true,
		MaxToolOutputChars: 8000,
	}
}

// ── Manager ──────────────────────────────────────────────────────────────────

// Manager manages session transcripts, compaction, and pruning.
type Manager struct {
	dir        string
	cfg        Config
	active     map[string]*Session // agentID → session
	memStore   MemoryFlusher       // optional — write compaction summaries to memory
	mu         sync.RWMutex
}

// MemoryFlusher is the minimal interface for persisting compaction summaries.
// Implemented by memory.Store — avoids circular dependency.
type MemoryFlusher interface {
	Put(path string, data []byte) error
}

// SetMemoryFlusher attaches a memory store for pre-compaction durable writes.
func (m *Manager) SetMemoryFlusher(ms MemoryFlusher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memStore = ms
}

// NewManager creates a session manager.
func NewManager(dir string, cfg Config) *Manager {
	os.MkdirAll(dir, 0755)
	return &Manager{
		dir:    dir,
		cfg:    cfg,
		active: make(map[string]*Session),
	}
}

// GetOrCreate returns the active session for an agent, creating one if needed.
func (m *Manager) GetOrCreate(agentID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.active[agentID]; ok {
		return s
	}
	// Try loading from disk
	s := m.load(agentID)
	if s == nil {
		s = &Session{
			ID:        newSessionID(agentID),
			AgentID:   agentID,
			CreatedAt: time.Now(),
		}
	}
	m.active[agentID] = s
	return s
}

// AddTurn records a turn and triggers auto-compaction if needed.
// Returns true if compaction ran.
func (m *Manager) AddTurn(agentID, role, content string, tokens int) (bool, error) {
	m.mu.RLock()
	s, ok := m.active[agentID]
	m.mu.RUnlock()
	if !ok {
		s = m.GetOrCreate(agentID)
	}

	s.mu.Lock()
	turn := Turn{
		Timestamp: time.Now(),
		Role:      role,
		Content:   content,
		Tokens:    tokens,
	}
	if role == "tool" {
		turn.ToolName = "tool"
	}
	s.Turns = append(s.Turns, turn)
	s.UpdatedAt = time.Now()
	totalTokens := s.totalTokens()
	s.mu.Unlock()

	// Check if we need to compact
	threshold := m.cfg.MaxContextTokens * m.cfg.AutoCompactAt / 100
	if totalTokens > threshold {
		return m.Compact(agentID)
	}
	return false, m.save(s)
}

// AddToolResult records a tool result (pruned if configured).
func (m *Manager) AddToolResult(agentID, toolName, output string, tokens int) (bool, error) {
	// Prune tool output if configured
	if m.cfg.PruneToolOutputs && len(output) > m.cfg.MaxToolOutputChars {
		output = output[:m.cfg.MaxToolOutputChars] +
			fmt.Sprintf("\n[... truncated %d chars by session pruning]", len(output)-m.cfg.MaxToolOutputChars)
	}
	return m.AddTurn(agentID, "tool", output, tokens)
}

// Compact summarizes older turns and replaces them with a summary.
// Keeps a tail of recent turns defined by cfg.KeepRecentTokens.
func (m *Manager) Compact(agentID string) (bool, error) {
	m.mu.RLock()
	s, ok := m.active[agentID]
	m.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("session not found: %s", agentID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Turns) < 4 {
		return false, nil // nothing meaningful to compact
	}

	// Walk from the end to find the split point (keepRecentTokens tail)
	tailTokens := 0
	splitIdx := len(s.Turns)
	for i := len(s.Turns) - 1; i >= 0; i-- {
		tailTokens += max(s.Turns[i].Tokens, tokenEstimate(s.Turns[i].Content))
		if tailTokens >= m.cfg.KeepRecentTokens {
			splitIdx = i
			break
		}
	}
	if splitIdx < 2 {
		return false, nil // not enough turns to summarize
	}

	// Build a summary of turns[0:splitIdx]
	toSummarize := s.Turns[:splitIdx]
	summary := buildSummary(toSummarize)

	// ── Memory flush: persist full pre-compaction context to MeMex ──
	// Nothing is lost — the summary stays in transcript for LLM context,
	// and the full detail goes to durable memory.
	m.flushToMemory(s, toSummarize, summary)

	// Build new transcript: summary + remaining tail
	newTurns := []Turn{
		{
			Timestamp: time.Now(),
			Role:      "system",
			Content:   summary,
			Tokens:    tokenEstimate(summary),
		},
	}
	newTurns = append(newTurns, s.Turns[splitIdx:]...)

	s.Turns = newTurns
	s.Compacted++
	s.UpdatedAt = time.Now()

	return true, m.save(s)
}

// NewSession starts a fresh session (equivalent to /new).
func (m *Manager) NewSession(agentID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.active[agentID]
	if old != nil {
		m.save(old) // persist old session
	}
	s := &Session{
		ID:        newSessionID(agentID),
		AgentID:   agentID,
		CreatedAt: time.Now(),
	}
	m.active[agentID] = s
	return s
}

// History returns a copy of the current user/assistant turns for an agent.
// Skips system/compaction turns. Safe to call from any goroutine.
func (m *Manager) History(agentID string) []Turn {
	s := m.GetOrCreate(agentID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Turn
	for _, t := range s.Turns {
		if t.Role == "user" || t.Role == "assistant" {
			out = append(out, t)
		}
	}
	return out
}

// Stats returns compacted count and token usage for an agent session.
func (m *Manager) Stats(agentID string) (compacted int, totalTokens int, turnCount int) {
	m.mu.RLock()
	s := m.active[agentID]
	m.mu.RUnlock()
	if s == nil {
		return 0, 0, 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Compacted, s.totalTokens(), len(s.Turns)
}

// ── Persistence ──────────────────────────────────────────────────────────────

func (m *Manager) path(agentID string) string {
	return filepath.Join(m.dir, fmt.Sprintf("%s.json", agentID))
}

func (m *Manager) load(agentID string) *Session {
	data, err := os.ReadFile(m.path(agentID))
	if err != nil {
		return nil
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

// flushToMemory persists the pre-compaction context to durable memory.
// Full conversation detail is preserved before it's summarized — nothing lost.
func (m *Manager) flushToMemory(s *Session, toSummarize []Turn, summary string) {
	m.mu.RLock()
	ms := m.memStore
	m.mu.RUnlock()
	if ms == nil {
		return
	}

	// Build a rich MeMex entry with full pre-compaction context
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Compaction checkpoint — %s — #%d\n\n",
		s.AgentID, s.Compacted+1))
	sb.WriteString(fmt.Sprintf("Compacted at: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Turns summarized: %d\n", len(toSummarize)))
	sb.WriteString(fmt.Sprintf("Session turns before: %d, after: %d (tail preserved)\n\n",
		len(s.Turns), len(s.Turns)-len(toSummarize)+1))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(summary)
	sb.WriteString("\n\n---\n\n## Full Pre-Compaction Transcript\n\n")

	for _, t := range toSummarize {
		sb.WriteString(fmt.Sprintf("**[%s]** _%s_\n%s\n\n",
			t.Timestamp.Format("15:04:05"), t.Role, t.Content))
	}

	memPath := fmt.Sprintf("sessions/%s/compaction-%03d.md", s.AgentID, s.Compacted+1)
	if err := ms.Put(memPath, []byte(sb.String())); err != nil {
		// Non-fatal — compaction still proceeds
		fmt.Fprintf(os.Stderr, "session: memory flush warning: %v\n", err)
	}
}

func (m *Manager) save(s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path(s.AgentID), data, 0644)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func newSessionID(agentID string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())))
	return fmt.Sprintf("sess-%x", h[:8])
}

func (s *Session) totalTokens() int {
	total := 0
	for _, t := range s.Turns {
		total += max(t.Tokens, tokenEstimate(t.Content))
	}
	return total
}

func tokenEstimate(s string) int {
	// Rough estimate: ~4 chars per token for English text
	n := len(s)
	if n == 0 {
		return 0
	}
	return max(1, n/4)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// buildSummary creates a concise summary from a set of turns.
// In production, this would call an LLM. Here, we produce a structured
// digest highlighting key exchanges, decisions, and tool calls.
func buildSummary(turns []Turn) string {
	if len(turns) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("📋 Session compaction summary\n\n")

	// Count by role
	userMsgs, assistantMsgs, toolMsgs := 0, 0, 0
	topics := make(map[string]int) // keyword frequency
	decisions := []string{}
	toolNames := make(map[string]int)

	keywords := []string{"agent", "pipeline", "config", "error", "build", "deploy",
		"commit", "memory", "skill", "tool", "model", "token", "prompt", "fix",
		"dashboard", "engine", "security", "API", "LLM", "overview"}

	for _, t := range turns {
		switch t.Role {
		case "user":
			userMsgs++
		case "assistant":
			assistantMsgs++
			if strings.Contains(strings.ToLower(t.Content), "fix") ||
				strings.Contains(strings.ToLower(t.Content), "applied") ||
				strings.Contains(strings.ToLower(t.Content), "done") {
				decisions = append(decisions, truncateLine(t.Content, 80))
			}
		case "tool":
			toolMsgs++
			if t.ToolName != "" {
				toolNames[t.ToolName]++
			}
		}
		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(t.Content), kw) {
				topics[kw]++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("- %d user messages, %d assistant responses, %d tool calls\n",
		userMsgs, assistantMsgs, toolMsgs))

	// Top topics
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range topics {
		if v >= 2 {
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })

	if len(sorted) > 0 {
		sb.WriteString("- Key topics: ")
		mentions := []string{}
		for i, kv := range sorted {
			if i >= 5 {
				break
			}
			mentions = append(mentions, fmt.Sprintf("%s (%d)", kv.k, kv.v))
		}
		sb.WriteString(strings.Join(mentions, ", "))
		sb.WriteString("\n")
	}

	// Tools used
	if len(toolNames) > 0 {
		sb.WriteString("- Tools used: ")
		tools := []string{}
		for name, count := range toolNames {
			tools = append(tools, fmt.Sprintf("%s (%d)", name, count))
		}
		sb.WriteString(strings.Join(tools, ", "))
		sb.WriteString("\n")
	}

	// Decisions / outcomes
	if len(decisions) > 0 {
		sb.WriteString("- Outcomes:\n")
		for i, d := range decisions {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... and %d more outcomes\n", len(decisions)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("  • %s\n", d))
		}
	}

	// Time range
	if len(turns) > 0 {
		sb.WriteString(fmt.Sprintf("- Time range: %s → %s\n",
			turns[0].Timestamp.Format("15:04"),
			turns[len(turns)-1].Timestamp.Format("15:04")))
	}

	return sb.String()
}

func truncateLine(s string, maxLen int) string {
	// Take first line
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[:idx]
	}
	if len(s) > maxLen {
		s = s[:maxLen-3] + "..."
	}
	return s
}