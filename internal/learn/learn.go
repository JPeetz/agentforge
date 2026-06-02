// Package learn — self-learning observer → extractor → generator pipeline.
//
// AgentForge agents watch user interactions, extract recurring patterns,
// and auto-generate SKILL.md files when confidence thresholds are met.
// No external NLP/ML dependencies — purely Go stdlib with word-overlap
// similarity and in-memory interaction tracking.
//
//	User Messages → Observer → Pattern Store → Extractor → Skill Generator → SKILL.md
//	                    ↑                                              |
//	                    └────────── Feedback Loop ────────────────────┘
package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
)

// ── Interaction ──────────────────────────────────────────────────────────────

// Interaction is a single user or agent action observed by the system.
type Interaction struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`   // "telegram", "discord", "dashboard", "api"
	UserID    string    `json:"user_id"`
	Message   string    `json:"message"`  // truncated to 500 chars
	Intent    string    `json:"intent"`   // "command", "question", "feedback", "correction"
	Context   string    `json:"context"`  // agent name, pipeline, page
	AgentID   string    `json:"agent_id,omitempty"`
}

// ── Observer ──────────────────────────────────────────────────────────────────

// Observer watches bus events and logs interactions.
type Observer struct {
	interactions []Interaction
	mu           sync.RWMutex
	maxSize      int
	logger       *slog.Logger
}

// NewObserver creates an observer with the given max interaction buffer size.
func NewObserver(maxSize int) *Observer {
	return &Observer{
		interactions: make([]Interaction, 0, maxSize),
		maxSize:      maxSize,
		logger:       slog.Default(),
	}
}

// Record adds an observation, truncating message to 500 chars and pruning
// old entries if the buffer exceeds maxSize.
func (o *Observer) Record(interaction Interaction) {
	if len(interaction.Message) > 500 {
		interaction.Message = interaction.Message[:500]
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.interactions) >= o.maxSize {
		// Drop oldest 20% to make room
		drop := o.maxSize / 5
		if drop < 1 {
			drop = 1
		}
		o.interactions = o.interactions[drop:]
	}

	o.interactions = append(o.interactions, interaction)
}

// Recent returns the last n interactions, sorted newest-first.
func (o *Observer) Recent(n int) []Interaction {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if n <= 0 || n > len(o.interactions) {
		n = len(o.interactions)
	}

	out := make([]Interaction, n)
	for i := 0; i < n; i++ {
		out[i] = o.interactions[len(o.interactions)-1-i]
	}
	return out
}

// Count returns the total number of recorded interactions.
func (o *Observer) Count() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.interactions)
}

// snapshot returns a safe copy of all interactions for extraction.
func (o *Observer) snapshot() []Interaction {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]Interaction, len(o.interactions))
	copy(out, o.interactions)
	return out
}

// ── Pattern ─────────────────────────────────────────────────────────────────────

// Pattern is a recurring interaction pattern extracted by the Extractor.
type Pattern struct {
	Name       string    `json:"name"`        // "daily-standup", "code-review-request"
	Triggers   []string  `json:"triggers"`    // phrases that trigger this pattern
	Frequency  float64   `json:"frequency"`   // occurrences per day
	Intent     string    `json:"intent"`      // what the user wants
	Steps      []string  `json:"steps"`       // typical agent response sequence
	Confidence float64   `json:"confidence"`  // 0.0–1.0
	LastSeen   time.Time `json:"last_seen"`
	Count      int       `json:"count"`       // total observed instances
}

// ── Extractor ─────────────────────────────────────────────────────────────────

// Extractor analyses interactions for recurring patterns using word-overlap
// similarity and time-window clustering.
type Extractor struct {
	patterns map[string]*Pattern
	mu       sync.RWMutex
}

// NewExtractor creates a pattern extractor.
func NewExtractor() *Extractor {
	return &Extractor{
		patterns: make(map[string]*Pattern),
	}
}

// Run analyses the provided interactions and extracts/updates patterns.
func (ex *Extractor) Run(interactions []Interaction) []Pattern {
	if len(interactions) < 2 {
		return nil
	}

	ex.mu.Lock()
	defer ex.mu.Unlock()

	now := time.Now()

	// Phase 1: cluster interactions by intent within time windows.
	// Use a sliding window of 5 minutes to group related turns.
	clusters := clusterByIntent(interactions, 5*time.Minute)

	// Phase 2: for each cluster of 2+ interactions, extract a pattern.
	newPatterns := make(map[string]*Pattern)

	for _, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}

		// Infer pattern name from the dominant intent and messages
		pattern := ex.extractPattern(cluster)
		if pattern == nil {
			continue
		}

		// Merge with existing patterns using the same name
		if existing, ok := ex.patterns[pattern.Name]; ok {
			merged := mergePatterns(existing, pattern)
			newPatterns[merged.Name] = merged
		} else {
			newPatterns[pattern.Name] = pattern
		}
	}

	// Also keep existing patterns, updating last-seen based on fresh data
	for name, p := range ex.patterns {
		if _, replaced := newPatterns[name]; !replaced {
			// Pattern wasn't seen in this extraction run — decay confidence
			stale := p
			hoursSince := now.Sub(stale.LastSeen).Hours()
			if hoursSince > 24 {
				stale.Confidence *= 0.9 // decay
			}
			newPatterns[name] = stale
		}
	}

	ex.patterns = newPatterns

	// Return sorted list
	return ex.List()
}

// extractPattern infers a pattern from a cluster of related interactions.
func (ex *Extractor) extractPattern(cluster []Interaction) *Pattern {
	if len(cluster) == 0 {
		return nil
	}

	// Find the most recent interaction for LastSeen
	lastSeen := cluster[0].Timestamp
	for _, in := range cluster {
		if in.Timestamp.After(lastSeen) {
			lastSeen = in.Timestamp
		}
	}

	// Determine dominant intent
	intentCounts := make(map[string]int)
	for _, in := range cluster {
		intentCounts[in.Intent]++
	}
	dominantIntent := ""
	maxCount := 0
	for intent, count := range intentCounts {
		if count > maxCount {
			maxCount = count
			dominantIntent = intent
		}
	}

	// Extract trigger phrases — words that appear in multiple messages
	triggers := extractTriggers(cluster)

	// Extract typical steps from context/tool patterns
	steps := extractSteps(cluster)

	// Build pattern name from intent + triggers
	name := buildPatternName(dominantIntent, triggers)

	// Compute confidence based on cluster size, repetition, and clarity
	confidence := computeConfidence(cluster)

	return &Pattern{
		Name:       name,
		Triggers:   triggers,
		Frequency:  computeFrequency(cluster),
		Intent:     dominantIntent,
		Steps:      steps,
		Confidence: confidence,
		LastSeen:   lastSeen,
		Count:      len(cluster),
	}
}

// List returns all extracted patterns, sorted by confidence descending.
func (ex *Extractor) List() []Pattern {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	out := make([]Pattern, 0, len(ex.patterns))
	for _, p := range ex.patterns {
		out = append(out, *p)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Confidence > out[j].Confidence
	})

	return out
}

// Count returns the number of extracted patterns.
func (ex *Extractor) Count() int {
	ex.mu.RLock()
	defer ex.mu.RUnlock()
	return len(ex.patterns)
}

// ── Skill Generator ──────────────────────────────────────────────────────────

// Generator auto-creates SKILL.md files from high-confidence patterns.
type Generator struct {
	outputDir string
	logger    *slog.Logger
}

// NewGenerator creates a skill generator that writes to outputDir.
func NewGenerator(outputDir string) *Generator {
	return &Generator{
		outputDir: outputDir,
		logger:    slog.Default(),
	}
}

// Generate creates a SKILL.md file for the given pattern.
// Returns the path to the generated file.
func (g *Generator) Generate(pattern Pattern) (string, error) {
	if err := os.MkdirAll(g.outputDir, 0o755); err != nil {
		return "", fmt.Errorf("learn: mkdir %s: %w", g.outputDir, err)
	}

	// Build a safe directory name from the pattern name
	dirName := sanitizeName(pattern.Name)
	skillDir := filepath.Join(g.outputDir, dirName)

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", fmt.Errorf("learn: mkdir %s: %w", skillDir, err)
	}

	content := renderSkillMD(pattern)
	skillPath := filepath.Join(skillDir, "SKILL.md")

	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("learn: write skill: %w", err)
	}

	g.logger.Info("auto-generated skill",
		slog.String("pattern", pattern.Name),
		slog.String("path", skillPath),
		slog.Float64("confidence", pattern.Confidence),
		slog.Int("observations", pattern.Count),
	)

	return skillPath, nil
}

// renderSkillMD produces the SKILL.md content from a pattern.
func renderSkillMD(p Pattern) string {
	// Build allowed-tools from observed steps
	tools := inferTools(p.Steps)
	toolList := strings.Join(quoteEach(tools), ", ")

	// Infer capability requirements from tools
	capReq := inferCapability(tools)
	capList := strings.Join(quoteEach(capReq), ", ")

	// Build trigger examples
	triggerExamples := strings.Join(p.Triggers, "; ")

	desc := fmt.Sprintf("Auto-generated from %d observed interactions. %s",
		p.Count, buildDescription(p))

	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: auto-%s\n", sanitizeName(p.Name)))
	b.WriteString(fmt.Sprintf("description: \"%s\"\n", desc))
	b.WriteString(fmt.Sprintf("when_to_use: \"%s\"\n", triggerExamples))
	if toolList != "" {
		b.WriteString(fmt.Sprintf("allowed-tools: [%s]\n", toolList))
	} else {
		b.WriteString("allowed-tools: [read, write]\n")
	}
	if capList != "" {
		b.WriteString(fmt.Sprintf("capability-required: [%s]\n", capList))
	}
	b.WriteString(fmt.Sprintf("version: 0.1.0-auto\n"))
	b.WriteString(fmt.Sprintf("author: AgentForge Learning Engine\n"))
	b.WriteString("tags: [auto-generated, learning]\n")
	b.WriteString("---\n\n")

	// Body
	b.WriteString(fmt.Sprintf("# %s\n\n", p.Name))
	b.WriteString(fmt.Sprintf("%s\n\n", desc))
	b.WriteString("## When to Use\n\n")
	b.WriteString(fmt.Sprintf("This skill activates when the user sends messages matching: %s\n\n", triggerExamples))
	b.WriteString("## Observed Pattern\n\n")
	b.WriteString(fmt.Sprintf("- **Intent:** %s\n", p.Intent))
	b.WriteString(fmt.Sprintf("- **Count:** %d observed instances\n", p.Count))
	b.WriteString(fmt.Sprintf("- **Confidence:** %.1f%%\n", p.Confidence*100))
	b.WriteString(fmt.Sprintf("- **Last Seen:** %s\n", p.LastSeen.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Frequency:** %.1f/day\n\n", p.Frequency))

	if len(p.Steps) > 0 {
		b.WriteString("## Typical Steps\n\n")
		for i, step := range p.Steps {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("This skill was automatically generated by the AgentForge Learning Engine. ")
	b.WriteString("Review and refine these instructions based on your needs.\n\n")
	b.WriteString("1. Recognize the trigger phrases listed above\n")
	b.WriteString("2. Follow the typical response steps if applicable\n")
	b.WriteString(fmt.Sprintf("3. Match the observed intent: %s\n", p.Intent))
	b.WriteString("4. Adapt to the user's current context while maintaining the learned pattern\n")

	return b.String()
}

// ── Stats ───────────────────────────────────────────────────────────────────

// Stats provides metrics about the learning system.
type Stats struct {
	InteractionsObserved int       `json:"interactions_observed"`
	PatternsExtracted    int       `json:"patterns_extracted"`
	SkillsGenerated      int       `json:"skills_generated"`
	TopPatterns          []Pattern `json:"top_patterns"`
	LastExtraction       time.Time `json:"last_extraction,omitempty"`
	LastGeneration       time.Time `json:"last_generation,omitempty"`
}

// ── Manager ──────────────────────────────────────────────────────────────────

// Manager orchestrates the Observer → Extractor → Generator pipeline.
type Manager struct {
	observer    *Observer
	extractor   *Extractor
	generator   *Generator
	bus         bus.Bus
	mu          sync.RWMutex
	logger      *slog.Logger
	enabled     bool
	skillsGen   int

	lastExtraction time.Time
	lastGeneration time.Time
}

// NewManager creates a new learning manager subscribed to the given bus.
func NewManager(b bus.Bus, outputDir string) *Manager {
	return &Manager{
		observer:  NewObserver(1000),
		extractor: NewExtractor(),
		generator: NewGenerator(outputDir),
		bus:       b,
		logger:    slog.Default(),
		enabled:   true,
	}
}

// Start begins observing bus events for learning.
// Subscribes to channel.*.message, agent.*.response, and dashboard.action topics.
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("learn: starting observer")

	// Subscribe to channel events (user messages from Telegram, Discord, etc.)
	chMsg, err := m.bus.Subscribe("channel.*.message", nil)
	if err != nil {
		return fmt.Errorf("learn: subscribe channel.*.message: %w", err)
	}

	// Subscribe to agent responses
	agResp, err := m.bus.Subscribe("agent.*.response", nil)
	if err != nil {
		return fmt.Errorf("learn: subscribe agent.*.response: %w", err)
	}

	// Subscribe to dashboard actions
	dbAct, err := m.bus.Subscribe("dashboard.action", nil)
	if err != nil {
		return fmt.Errorf("learn: subscribe dashboard.action: %w", err)
	}

	go m.consume(ctx, chMsg, "channel.*.message")
	go m.consume(ctx, agResp, "agent.*.response")
	go m.consume(ctx, dbAct, "dashboard.action")

	m.logger.Info("learn: observer started",
		slog.Int("max_interactions", m.observer.maxSize),
		slog.String("output_dir", m.generator.outputDir),
	)

	return nil
}

// consume reads from a subscription channel and records interactions.
func (m *Manager) consume(ctx context.Context, ch <-chan bus.Envelope, topic string) {
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-ch:
			if !ok {
				return
			}
			if !m.enabled {
				continue
			}

			interaction := m.parseEnvelope(env, topic)
			m.Record(interaction)
		}
	}
}

// parseEnvelope converts a bus envelope into an Interaction.
func (m *Manager) parseEnvelope(env bus.Envelope, topic string) Interaction {
	in := Interaction{
		Timestamp: env.Timestamp,
		Source:    env.Source,
		Intent:    inferIntent(topic),
		Context:   topic,
	}

	// Try to extract message from payload
	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err == nil {
		if msg, ok := payload["message"].(string); ok {
			in.Message = msg
		}
		if txt, ok := payload["text"].(string); ok {
			if in.Message == "" {
				in.Message = txt
			}
		}
		if uid, ok := payload["user_id"].(string); ok {
			in.UserID = uid
		}
		if aid, ok := payload["agent_id"].(string); ok {
			in.AgentID = aid
		}
		if intent, ok := payload["intent"].(string); ok && intent != "" {
			in.Intent = intent
		}
		if ctx, ok := payload["context"].(string); ok {
			in.Context = ctx
		}
	}

	// Fallback: try message as raw string
	if in.Message == "" {
		in.Message = string(env.Payload)
	}

	if in.UserID == "" {
		in.UserID = env.Source
	}

	return in
}

// Record adds an interaction to the observer.
func (m *Manager) Record(interaction Interaction) {
	m.observer.Record(interaction)
}

// Extract runs pattern extraction on all observed interactions.
func (m *Manager) Extract() []Pattern {
	m.mu.Lock()
	defer m.mu.Unlock()

	interactions := m.observer.snapshot()
	patterns := m.extractor.Run(interactions)
	m.lastExtraction = time.Now()
	return patterns
}

// Generate creates SKILL.md files for all patterns meeting the threshold:
// confidence > 0.8 AND count >= 5.
func (m *Manager) Generate() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First extract to get fresh patterns
	interactions := m.observer.snapshot()
	patterns := m.extractor.Run(interactions)
	m.lastExtraction = time.Now()

	var generated []string
	for _, p := range patterns {
		if p.Confidence > 0.8 && p.Count >= 5 {
			path, err := m.generator.Generate(p)
			if err != nil {
				m.logger.Warn("learn: generate failed",
					slog.String("pattern", p.Name),
					slog.Any("error", err),
				)
				continue
			}
			generated = append(generated, path)
			m.skillsGen++
		}
	}

	m.lastGeneration = time.Now()
	return generated, nil
}

// AutoGenerate runs full Extract → Generate pipeline for eligible patterns.
func (m *Manager) AutoGenerate() ([]Pattern, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	interactions := m.observer.snapshot()
	patterns := m.extractor.Run(interactions)
	m.lastExtraction = time.Now()

	var eligible []Pattern
	var generated []string

	for _, p := range patterns {
		if p.Confidence > 0.8 && p.Count >= 5 {
			eligible = append(eligible, p)
			path, err := m.generator.Generate(p)
			if err != nil {
				m.logger.Warn("learn: generate failed",
					slog.String("pattern", p.Name),
					slog.Any("error", err),
				)
				continue
			}
			generated = append(generated, path)
			m.skillsGen++
		}
	}

	m.lastGeneration = time.Now()
	return eligible, generated
}

// Stats returns current learning system metrics.
func (m *Manager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patterns := m.extractor.List()
	// Top 5 patterns by confidence
	top := patterns
	if len(top) > 5 {
		top = top[:5]
	}

	return Stats{
		InteractionsObserved: m.observer.Count(),
		PatternsExtracted:    m.extractor.Count(),
		SkillsGenerated:      m.skillsGen,
		TopPatterns:          top,
		LastExtraction:       m.lastExtraction,
		LastGeneration:       m.lastGeneration,
	}
}

// SetEnabled toggles learning on/off.
func (m *Manager) SetEnabled(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = v
}

// IsEnabled returns whether learning is active.
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Observer returns the underlying observer for dashboard access.
func (m *Manager) Observer() *Observer { return m.observer }

// Extractor returns the underlying extractor.
func (m *Manager) Extractor() *Extractor { return m.extractor }

// Generator returns the underlying generator.
func (m *Manager) Generator() *Generator { return m.generator }

// ── Clustering ───────────────────────────────────────────────────────────────

// clusterByIntent groups interactions by intent within sliding time windows.
// Interactions with the same intent within the window are clustered together.
func clusterByIntent(interactions []Interaction, window time.Duration) [][]Interaction {
	if len(interactions) == 0 {
		return nil
	}

	type group struct {
		intent  string
		members []Interaction
	}

	var groups []*group

	for _, in := range interactions {
		found := false
		for _, g := range groups {
			if g.intent == in.Intent {
				// Check if within time window of the most recent member
				last := g.members[len(g.members)-1]
				if in.Timestamp.Sub(last.Timestamp) <= window {
					g.members = append(g.members, in)
					found = true
					break
				}
			}
		}
		if !found {
			// Also check similarity across intents for cross-intent clustering
			for _, g := range groups {
				if wordsSimilar(g.members[len(g.members)-1].Message, in.Message) > 0.4 {
					last := g.members[len(g.members)-1]
					if in.Timestamp.Sub(last.Timestamp) <= window {
						g.members = append(g.members, in)
						g.intent = in.Intent // update to more promising intent
						found = true
						break
					}
				}
			}
		}
		if !found {
			groups = append(groups, &group{intent: in.Intent, members: []Interaction{in}})
		}
	}

	out := make([][]Interaction, 0, len(groups))
	for _, g := range groups {
		if len(g.members) >= 2 {
			out = append(out, g.members)
		}
	}

	return out
}

// wordsSimilar computes simple word-overlap similarity (Jaccard coefficient).
// Returns 0.0–1.0, with 0.4+ indicating meaningful similarity.
func wordsSimilar(a, b string) float64 {
	wordsA := tokenize(a)
	wordsB := tokenize(b)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	// Intersection
	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}

	intersection := 0
	seen := make(map[string]bool)
	for _, w := range wordsB {
		if setA[w] && !seen[w] {
			intersection++
			seen[w] = true
		}
	}

	// Union size
	union := make(map[string]bool)
	for _, w := range wordsA {
		union[w] = true
	}
	for _, w := range wordsB {
		union[w] = true
	}

	if len(union) == 0 {
		return 0
	}

	return float64(intersection) / float64(len(union))
}

// tokenize splits a string into lowercase word tokens, removing stop words
// and short tokens.
func tokenize(s string) []string {
	// Remove common punctuation, lowercase, split
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' {
			return r
		}
		return ' '
	}, s)

	raw := strings.Fields(s)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "up": true, "down": true,
		"out": true, "off": true, "over": true, "under": true, "again": true,
		"further": true, "then": true, "once": true, "here": true, "there": true,
		"when": true, "where": true, "why": true, "how": true, "all": true,
		"both": true, "each": true, "few": true, "more": true, "most": true,
		"other": true, "some": true, "such": true, "no": true, "nor": true,
		"not": true, "only": true, "own": true, "same": true, "so": true,
		"than": true, "too": true, "very": true, "just": true, "don": true,
		"now": true, "it": true, "its": true, "this": true, "that": true,
		"these": true, "those": true, "my": true, "your": true, "our": true,
		"their": true, "i": true, "me": true, "you": true, "he": true,
		"she": true, "him": true, "her": true, "we": true, "they": true,
		"them": true, "what": true, "which": true, "who": true, "whom": true,
		"and": true, "but": true, "or": true, "if": true, "because": true,
		"about": true, "get": true, "got": true, "want": true, "need": true,
		"like": true, "know": true, "think": true, "say": true, "see": true,
		"well": true, "also": true, "any": true, "even": true, "still": true,
		"way": true, "make": true, "really": true, "much": true, "go": true,
	}

	var out []string
	for _, w := range raw {
		if len(w) < 2 {
			continue
		}
		if stopWords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// ── Trigger Extraction ──────────────────────────────────────────────────────

// extractTriggers finds words/phrases that appear in multiple messages
// within a cluster, indicating a common trigger pattern.
func extractTriggers(cluster []Interaction) []string {
	if len(cluster) < 2 {
		return nil
	}

	// Build word frequency across the cluster
	wordFreq := make(map[string]int)
	for _, in := range cluster {
		tokens := tokenize(in.Message)
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				wordFreq[t]++
				seen[t] = true
			}
		}
	}

	// Also look for 2-word and 3-word phrases
	phraseFreq := make(map[string]int)
	for _, in := range cluster {
		tokens := tokenize(in.Message)
		seen := make(map[string]bool)
		for i := 0; i < len(tokens)-1; i++ {
			bigram := tokens[i] + " " + tokens[i+1]
			if !seen[bigram] {
				phraseFreq[bigram]++
				seen[bigram] = true
			}
			// Also keep single words for frequency
			if _, ok := phraseFreq[tokens[i]]; !ok {
				phraseFreq[tokens[i]] = 0 // mark as seen through wordFreq
			}
		}
	}

	threshold := len(cluster) // must appear in at least N interactions
	if threshold > 2 {
		threshold = 2
	}
	if threshold < 1 {
		threshold = 1
	}

	type entry struct {
		word  string
		count int
	}

	var candidates []entry
	for w, c := range phraseFreq {
		if c >= threshold && len(w) > 2 {
			candidates = append(candidates, entry{w, c})
		}
	}
	// Also include top single words
	for w, c := range wordFreq {
		if c >= threshold && len(w) > 2 {
			// Don't duplicate if already in phrases
			dup := false
			for _, e := range candidates {
				if e.word == w {
					dup = true
					break
				}
			}
			if !dup {
				candidates = append(candidates, entry{w, c})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})

	// Take top 5 trigger phrases
	limit := 5
	if len(candidates) < limit {
		limit = len(candidates)
	}

	triggers := make([]string, limit)
	for i := 0; i < limit; i++ {
		triggers[i] = candidates[i].word
	}

	return triggers
}

// ── Step Extraction ─────────────────────────────────────────────────────────

// extractSteps extracts typical operation sequences from a cluster.
func extractSteps(cluster []Interaction) []string {
	stepFreq := make(map[string]int)
	stepOrder := make([]string, 0)

	for _, in := range cluster {
		ctx := strings.ToLower(in.Context)
		if ctx == "" {
			continue
		}
		if _, exists := stepFreq[ctx]; !exists {
			stepOrder = append(stepOrder, ctx)
		}
		stepFreq[ctx]++
	}

	// Also check for tool-calling patterns in the message
	for _, in := range cluster {
		msg := strings.ToLower(in.Message)
		for _, tool := range []string{"search", "read", "write", "fetch", "generate", "analyze", "review", "audit", "compile", "deploy"} {
			if strings.Contains(msg, tool) {
				stepFreq[tool]++
				alreadyInOrder := false
				for _, s := range stepOrder {
					if s == tool {
						alreadyInOrder = true
						break
					}
				}
				if !alreadyInOrder {
					stepOrder = append(stepOrder, tool)
				}
			}
		}
	}

	// Return top steps by frequency, maintaining rough order
	type stepE struct {
		name  string
		count int
		order int
	}

	var ranked []stepE
	for i, name := range stepOrder {
		ranked = append(ranked, stepE{name, stepFreq[name], i})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count ||
			(ranked[i].count == ranked[j].count && ranked[i].order < ranked[j].order)
	})

	limit := 5
	if len(ranked) < limit {
		limit = len(ranked)
	}

	steps := make([]string, limit)
	for i := 0; i < limit; i++ {
		steps[i] = ranked[i].name
	}

	return steps
}

// ── Name & Description ─────────────────────────────────────────────────────

// buildPatternName creates a human-readable pattern name from intent + triggers.
func buildPatternName(intent string, triggers []string) string {
	parts := []string{}

	if intent != "" && intent != "unknown" {
		parts = append(parts, intent)
	}

	for _, t := range triggers {
		if len(parts) >= 2 {
			break
		}
		parts = append(parts, t)
	}

	if len(parts) == 0 {
		return "unknown-pattern"
	}

	return strings.Join(parts, "-")
}

// buildDescription creates a natural-language description of the pattern.
func buildDescription(p Pattern) string {
	if len(p.Triggers) > 0 {
		return fmt.Sprintf("Detected when user says phrases like '%s' with intent '%s'.",
			strings.Join(p.Triggers, "', '"), p.Intent)
	}
	return fmt.Sprintf("Detected repeated interactions with intent '%s'.", p.Intent)
}

// ── Confidence ──────────────────────────────────────────────────────────────

// computeConfidence computes a confidence score (0.0–1.0) for a cluster.
// Factors: cluster size, word similarity coherence, intent agreement, recency.
func computeConfidence(cluster []Interaction) float64 {
	if len(cluster) < 2 {
		return 0.1
	}

	n := float64(len(cluster))

	// Base from cluster size (saturates at 10)
	sizeScore := math.Min(n/10.0, 1.0)

	// Word similarity coherence — average pairwise Jaccard
	var simSum float64
	var simCount int
	for i := 0; i < len(cluster); i++ {
		for j := i + 1; j < len(cluster); j++ {
			simSum += wordsSimilar(cluster[i].Message, cluster[j].Message)
			simCount++
		}
	}
	simScore := 0.0
	if simCount > 0 {
		simScore = simSum / float64(simCount)
	}

	// Intent agreement
	intentCounts := make(map[string]int)
	for _, in := range cluster {
		intentCounts[in.Intent]++
	}
	maxIntent := 0
	for _, c := range intentCounts {
		if c > maxIntent {
			maxIntent = c
		}
	}
	intentScore := float64(maxIntent) / n

	// Recency boost — interactions within last 24 hours
	now := time.Now()
	recentCount := 0
	for _, in := range cluster {
		if now.Sub(in.Timestamp) < 24*time.Hour {
			recentCount++
		}
	}
	recencyScore := float64(recentCount) / n

	// Weighted combination
	conf := 0.30*sizeScore + 0.35*simScore + 0.20*intentScore + 0.15*recencyScore

	// Clamp
	if conf > 1.0 {
		conf = 1.0
	}
	if conf < 0.0 {
		conf = 0.0
	}

	return conf
}

// computeFrequency estimates occurrences per day based on cluster timestamps.
func computeFrequency(cluster []Interaction) float64 {
	if len(cluster) < 2 {
		return 0
	}

	first := cluster[0].Timestamp
	last := cluster[len(cluster)-1].Timestamp
	span := last.Sub(first)

	if span < time.Hour {
		// Short span — extrapolate
		spanHours := math.Max(span.Hours(), 0.1)
		return float64(len(cluster)) / spanHours * 24
	}

	days := span.Hours() / 24
	if days < 0.01 {
		days = 0.01
	}

	return float64(len(cluster)) / days
}

// ── Pattern Merging ─────────────────────────────────────────────────────────

// mergePatterns combines an existing pattern with new observations.
func mergePatterns(existing, new *Pattern) *Pattern {
	merged := *existing

	// Merge triggers (keep unique, limit 5)
	triggerSet := make(map[string]bool)
	for _, t := range existing.Triggers {
		triggerSet[t] = true
	}
	for _, t := range new.Triggers {
		triggerSet[t] = true
	}
	merged.Triggers = make([]string, 0, len(triggerSet))
	for t := range triggerSet {
		merged.Triggers = append(merged.Triggers, t)
	}
	sort.Strings(merged.Triggers)
	if len(merged.Triggers) > 5 {
		merged.Triggers = merged.Triggers[:5]
	}

	// Weighted confidence update (new observations get 0.3 weight)
	merged.Confidence = existing.Confidence*0.7 + new.Confidence*0.3

	// Update LastSeen to most recent
	if new.LastSeen.After(existing.LastSeen) {
		merged.LastSeen = new.LastSeen
	}

	// Accumulate count
	merged.Count += new.Count

	// Recompute frequency
	merged.Frequency = (existing.Frequency*0.7 + new.Frequency*0.3)

	// Merge steps
	stepSet := make(map[string]bool)
	var mergedSteps []string
	for _, s := range existing.Steps {
		if !stepSet[s] {
			stepSet[s] = true
			mergedSteps = append(mergedSteps, s)
		}
	}
	for _, s := range new.Steps {
		if !stepSet[s] {
			stepSet[s] = true
			mergedSteps = append(mergedSteps, s)
		}
	}
	if len(mergedSteps) > 5 {
		mergedSteps = mergedSteps[:5]
	}
	merged.Steps = mergedSteps

	return &merged
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// inferIntent maps a bus topic to an interaction intent.
func inferIntent(topic string) string {
	switch {
	case strings.Contains(topic, "channel"):
		return "command"
	case strings.Contains(topic, "agent"):
		return "response"
	case strings.Contains(topic, "dashboard"):
		return "feedback"
	default:
		return "unknown"
	}
}

// sanitizeName converts a pattern name to a safe filesystem name.
func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)

	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Trim leading/trailing hyphens and dots
	s = strings.Trim(s, "-.")
	if s == "" {
		s = "auto-skill"
	}
	return s
}

// inferTools maps observed step names to likely tool names.
func inferTools(steps []string) []string {
	toolMap := map[string]string{
		"search":   "web_search",
		"read":     "read",
		"write":    "write",
		"fetch":    "web_fetch",
		"generate": "image_generate",
		"analyze":  "web_fetch",
		"review":   "read",
		"audit":    "web_search",
		"compile":  "write",
		"deploy":   "write",
	}

	seen := make(map[string]bool)
	var tools []string
	for _, step := range steps {
		if tool, ok := toolMap[step]; ok {
			if !seen[tool] {
				tools = append(tools, tool)
				seen[tool] = true
			}
		}
	}
	return tools
}

// inferCapability maps tools to required capabilities.
func inferCapability(tools []string) []string {
	capMap := map[string]bool{
		"web_search":       true,
		"web_fetch":        true,
		"image_generate":   true,
	}

	capNeeds := make(map[string]bool)
	for _, t := range tools {
		if capMap[t] {
			capNeeds["network"] = true
		}
	}

	// Filesystem always needed for read/write
	for _, t := range tools {
		if t == "read" || t == "write" {
			capNeeds["filesystem"] = true
		}
	}

	var caps []string
	for c := range capNeeds {
		caps = append(caps, c)
	}
	sort.Strings(caps)
	return caps
}

// quoteEach wraps each string in a slice with double quotes.
func quoteEach(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}