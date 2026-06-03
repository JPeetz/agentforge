package learn

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
)

// ── Observer Tests ──────────────────────────────────────────────────────────

func TestObserver_NewObserver(t *testing.T) {
	obs := NewObserver(500)
	if obs == nil {
		t.Fatal("Observer should not be nil")
	}
	if obs.Count() != 0 {
		t.Errorf("New observer should have 0 interactions, got %d", obs.Count())
	}
}

func TestObserver_Record(t *testing.T) {
	obs := NewObserver(100)

	interaction := Interaction{
		Timestamp: time.Now(),
		Source:    "test",
		UserID:    "user-1",
		Message:   "hello",
		Intent:    "command",
		Context:   "test-topic",
	}

	obs.Record(interaction)

	if obs.Count() != 1 {
		t.Errorf("Expected 1 interaction, got %d", obs.Count())
	}
}

func TestObserver_MessageTruncation(t *testing.T) {
	obs := NewObserver(100)

	longMessage := strings.Repeat("x", 600) // 600 chars

	interaction := Interaction{
		Timestamp: time.Now(),
		Source:    "test",
		UserID:    "user-1",
		Message:   longMessage,
		Intent:    "command",
	}

	obs.Record(interaction)

	recent := obs.Recent(1)
	if len(recent) != 1 {
		t.Fatal("Should have 1 interaction")
	}

	if len(recent[0].Message) != 500 {
		t.Errorf("Expected message length 500, got %d", len(recent[0].Message))
	}
}

func TestObserver_BufferOverflow(t *testing.T) {
	obs := NewObserver(10) // Small buffer

	// Add 20 interactions (exceed buffer)
	for i := 0; i < 20; i++ {
		obs.Record(Interaction{
			Timestamp: time.Now(),
			Source:    "test",
			UserID:    "user-1",
			Message:   "msg",
			Intent:    "command",
		})
	}

	// Should keep only 10 (or close to it due to 20% drop)
	count := obs.Count()
	if count > 12 { // Allow some tolerance
		t.Errorf("Expected buffer to stay near 10, got %d", count)
	}
}

func TestObserver_Recent(t *testing.T) {
	obs := NewObserver(100)

	// Add 5 interactions with different timestamps
	for i := 0; i < 5; i++ {
		obs.Record(Interaction{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Source:    "test",
			UserID:    "user-1",
			Message:   "msg",
			Intent:    "command",
		})
	}

	// Get last 3 (newest first)
	recent := obs.Recent(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 interactions, got %d", len(recent))
	}

	// Should be newest first (reverse order)
	for i := 0; i < len(recent)-1; i++ {
		if recent[i].Timestamp.Before(recent[i+1].Timestamp) {
			t.Error("Recent should return newest first")
		}
	}
}

// ── Pattern Tests ───────────────────────────────────────────────────────────

func TestPattern_Structure(t *testing.T) {
	pattern := Pattern{
		Name:       "test-pattern",
		Triggers:   []string{"hello", "hi"},
		Frequency:  2.5,
		Intent:     "command",
		Steps:      []string{"read", "write"},
		Confidence: 0.85,
		Count:      10,
	}

	if pattern.Name != "test-pattern" {
		t.Errorf("Expected name 'test-pattern', got %q", pattern.Name)
	}
	if pattern.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", pattern.Confidence)
	}
}

// ── Extractor Tests ─────────────────────────────────────────────────────────

func TestExtractor_NewExtractor(t *testing.T) {
	ex := NewExtractor()
	if ex == nil {
		t.Fatal("Extractor should not be nil")
	}
	if ex.Count() != 0 {
		t.Errorf("New extractor should have 0 patterns, got %d", ex.Count())
	}
}

func TestExtractor_RunWithFewInteractions(t *testing.T) {
	ex := NewExtractor()

	// Less than 2 interactions should return nil
	interactions := []Interaction{
		{
			Timestamp: time.Now(),
			Source:    "test",
			Message:   "hello",
			Intent:    "command",
		},
	}

	patterns := ex.Run(interactions)
	if patterns != nil && len(patterns) > 0 {
		t.Error("Should return no patterns for < 2 interactions")
	}
}

func TestExtractor_ListPatterns(t *testing.T) {
	ex := NewExtractor()

	// List with no patterns
	patterns := ex.List()
	if patterns == nil || len(patterns) != 0 {
		t.Errorf("Expected empty patterns, got %d", len(patterns))
	}
}

func TestExtractor_CountPatterns(t *testing.T) {
	ex := NewExtractor()

	count := ex.Count()
	if count != 0 {
		t.Errorf("Expected 0 patterns, got %d", count)
	}
}

// ── Generator Tests ─────────────────────────────────────────────────────────

func TestGenerator_NewGenerator(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir)
	if gen == nil {
		t.Fatal("Generator should not be nil")
	}
}

func TestGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir)

	pattern := Pattern{
		Name:       "test-pattern",
		Triggers:   []string{"hello", "hi"},
		Frequency:  2.5,
		Intent:     "command",
		Steps:      []string{"search", "analyze"},
		Confidence: 0.85,
		Count:      10,
		LastSeen:   time.Now(),
	}

	path, err := gen.Generate(pattern)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Generated file does not exist: %s", path)
	}

	// Check content
	content, _ := os.ReadFile(path)
	contentStr := string(content)

	if !strings.Contains(contentStr, "auto-test-pattern") {
		t.Error("SKILL.md should contain pattern name")
	}
	if !strings.Contains(contentStr, "---") {
		t.Error("SKILL.md should have frontmatter")
	}
	if !strings.Contains(contentStr, "hello") {
		t.Error("SKILL.md should contain trigger phrases")
	}
}

func TestGenerator_GenerateCreatesValidSKILL(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir)

	pattern := Pattern{
		Name:       "database-query",
		Triggers:   []string{"query", "database"},
		Frequency:  1.5,
		Intent:     "command",
		Steps:      []string{"read", "analyze"},
		Confidence: 0.92,
		Count:      15,
		LastSeen:   time.Now(),
	}

	path, err := gen.Generate(pattern)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	contentStr := string(content)

	// Verify frontmatter
	lines := strings.Split(contentStr, "\n")
	if lines[0] != "---" || !strings.Contains(contentStr, "---\n") {
		t.Error("Missing valid YAML frontmatter")
	}

	// Verify required fields
	if !strings.Contains(contentStr, "name: auto-") {
		t.Error("Missing 'name' field")
	}
	if !strings.Contains(contentStr, "description:") {
		t.Error("Missing 'description' field")
	}
	if !strings.Contains(contentStr, "allowed-tools:") {
		t.Error("Missing 'allowed-tools' field")
	}
}

// ── Manager Tests ───────────────────────────────────────────────────────────

func TestManager_NewManager(t *testing.T) {
	tmpDir := t.TempDir()
	mockBus := bus.NewLocal()
	defer mockBus.Close()

	mgr := NewManager(mockBus, tmpDir)
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
	if !mgr.IsEnabled() {
		t.Error("Manager should be enabled by default")
	}
}

func TestManager_Record(t *testing.T) {
	tmpDir := t.TempDir()
	mockBus := bus.NewLocal()
	defer mockBus.Close()

	mgr := NewManager(mockBus, tmpDir)

	interaction := Interaction{
		Timestamp: time.Now(),
		Source:    "test",
		UserID:    "user-1",
		Message:   "hello",
		Intent:    "command",
	}

	mgr.Record(interaction)

	if mgr.Observer().Count() != 1 {
		t.Errorf("Expected 1 interaction, got %d", mgr.Observer().Count())
	}
}

func TestManager_GetObserverExtractorGenerator(t *testing.T) {
	tmpDir := t.TempDir()
	mockBus := bus.NewLocal()
	defer mockBus.Close()

	mgr := NewManager(mockBus, tmpDir)

	// Verify accessors work
	if mgr.Observer() == nil {
		t.Error("Observer should not be nil")
	}
	if mgr.Extractor() == nil {
		t.Error("Extractor should not be nil")
	}
	if mgr.Generator() == nil {
		t.Error("Generator should not be nil")
	}
}

func TestManager_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	mockBus := bus.NewLocal()
	defer mockBus.Close()

	mgr := NewManager(mockBus, tmpDir)

	if !mgr.IsEnabled() {
		t.Error("Manager should be enabled by default")
	}

	mgr.SetEnabled(false)
	if mgr.IsEnabled() {
		t.Error("Manager should be disabled")
	}

	mgr.SetEnabled(true)
	if !mgr.IsEnabled() {
		t.Error("Manager should be enabled")
	}
}

// ── Concurrent Tests ────────────────────────────────────────────────────────

func TestObserver_ConcurrentRecord(t *testing.T) {
	obs := NewObserver(1000)

	numWorkers := 10
	messagesPerWorker := 20

	done := make(chan bool, numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			for m := 0; m < messagesPerWorker; m++ {
				obs.Record(Interaction{
					Timestamp: time.Now(),
					Source:    "test",
					UserID:    "user-1",
					Message:   "test message",
					Intent:    "command",
				})
			}
			done <- true
		}(w)
	}

	// Wait for all workers
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	expected := numWorkers * messagesPerWorker
	if obs.Count() < expected-20 { // Allow some buffer drops
		t.Errorf("Expected at least %d interactions, got %d", expected-20, obs.Count())
	}
}

func TestExtractor_ConcurrentAccess(t *testing.T) {
	ex := NewExtractor()

	done := make(chan bool, 5)

	// Concurrent list operations
	for w := 0; w < 5; w++ {
		go func() {
			_ = ex.List() // Safe to call concurrently
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

// ── Integration Tests ───────────────────────────────────────────────────────

func TestLearnPipeline_GeneratorWithPattern(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewGenerator(tmpDir)

	// Create a realistic pattern
	pattern := Pattern{
		Name:       "user-search-query",
		Triggers:   []string{"search", "find"},
		Frequency:  3.2,
		Intent:     "command",
		Steps:      []string{"search", "analyze"},
		Confidence: 0.88,
		Count:      12,
		LastSeen:   time.Now(),
	}

	// Generate skill
	path, err := gen.Generate(pattern)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify SKILL.md was created
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read generated SKILL.md: %v", err)
	}

	skillContent := string(content)
	if !strings.Contains(skillContent, "---") {
		t.Error("SKILL.md missing frontmatter")
	}
	if !strings.Contains(skillContent, "name:") {
		t.Error("SKILL.md missing name field")
	}
	if !strings.Contains(skillContent, "allowed-tools:") {
		t.Error("SKILL.md missing allowed-tools field")
	}
}

// ── Helper Tests ────────────────────────────────────────────────────────────

func TestWordsSimilar(t *testing.T) {
	tests := []struct {
		a, b     string
		minScore float64
	}{
		{"hello world", "hello there", 0.2},
		{"hello world", "hello world", 1.0},
		{"hello world", "goodbye moon", 0.0},
		{"search database", "search query", 0.2},
	}

	for _, test := range tests {
		score := wordsSimilar(test.a, test.b)
		if score < test.minScore {
			t.Errorf("wordsSimilar(%q, %q) = %f, expected >= %f", test.a, test.b, score, test.minScore)
		}
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("hello world from golang")
	if len(tokens) < 3 {
		t.Errorf("Expected at least 3 tokens, got %d", len(tokens))
	}

	// Stop words should be removed
	tokens2 := tokenize("the quick brown fox")
	foundThe := false
	for _, t := range tokens2 {
		if t == "the" {
			foundThe = true
		}
	}
	if foundThe {
		t.Error("Stop word 'the' should be removed")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Test Pattern", "test-pattern"},
		{"hello-world", "hello-world"},
		{"hello_world", "hello_world"},
		{"hello world!", "hello-world"},
		{"UPPERCASE", "uppercase"},
	}

	for _, test := range tests {
		result := sanitizeName(test.input)
		if result != test.expected {
			t.Errorf("sanitizeName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestInferTools(t *testing.T) {
	steps := []string{"search", "read", "write", "analyze"}
	tools := inferTools(steps)

	if len(tools) == 0 {
		t.Error("Should infer tools from steps")
	}

	// "search" should map to "web_search"
	hasSearch := false
	for _, tool := range tools {
		if tool == "web_search" {
			hasSearch = true
		}
	}
	if !hasSearch {
		t.Error("Should infer 'web_search' from 'search' step")
	}
}

func TestInferCapability(t *testing.T) {
	tools := []string{"read", "write", "web_search"}
	caps := inferCapability(tools)

	if len(caps) == 0 {
		t.Error("Should infer capabilities from tools")
	}

	// "filesystem" should be needed for read/write
	hasFilesystem := false
	for _, cap := range caps {
		if cap == "filesystem" {
			hasFilesystem = true
		}
	}
	if !hasFilesystem {
		t.Error("Should infer 'filesystem' capability from read/write tools")
	}
}
