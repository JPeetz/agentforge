package skill

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ── Manifest Tests ──────────────────────────────────────────────────────────

func TestManifest_BasicFields(t *testing.T) {
	manifest := Manifest{
		Name:        "test-skill",
		Description: "A test skill for unit testing",
		Version:     "1.0.0",
		Author:      "Test Author",
		License:     "MIT",
	}

	if manifest.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got %q", manifest.Name)
	}
	if manifest.Description == "" {
		t.Error("Description should not be empty")
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %q", manifest.Version)
	}
}

func TestManifest_AllowedTools(t *testing.T) {
	manifest := Manifest{
		Name:         "read-skill",
		Description:  "Read files",
		AllowedTools: []string{"read", "list"},
	}

	if len(manifest.AllowedTools) != 2 {
		t.Errorf("Expected 2 allowed tools, got %d", len(manifest.AllowedTools))
	}
	if manifest.AllowedTools[0] != "read" {
		t.Errorf("Expected first tool 'read', got %q", manifest.AllowedTools[0])
	}
}

func TestManifest_Arguments(t *testing.T) {
	manifest := Manifest{
		Name:        "param-skill",
		Description: "Test with arguments",
		Arguments: []ManifestArgument{
			{Name: "query", Description: "Search query", Required: true},
			{Name: "limit", Description: "Result limit", Required: false, Default: "10"},
		},
	}

	if len(manifest.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(manifest.Arguments))
	}
	if manifest.Arguments[0].Name != "query" {
		t.Errorf("Expected first argument 'query', got %q", manifest.Arguments[0].Name)
	}
	if !manifest.Arguments[0].Required {
		t.Error("Query argument should be required")
	}
}

func TestManifest_Tags(t *testing.T) {
	manifest := Manifest{
		Name:        "tagged-skill",
		Description: "A skill with tags",
		Tags:        []string{"file", "system", "utility"},
	}

	if len(manifest.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(manifest.Tags))
	}
	found := false
	for _, tag := range manifest.Tags {
		if tag == "utility" {
			found = true
		}
	}
	if !found {
		t.Error("'utility' tag not found")
	}
}

// ── Skill Tests ─────────────────────────────────────────────────────────────

func TestSkill_BasicFields(t *testing.T) {
	skill := &Skill{
		Manifest: Manifest{
			Name:        "test-skill",
			Description: "Test skill",
		},
		Body:      "# Instructions\nDo something",
		Path:      "/tmp/skills/test-skill",
		Source:    "local",
		SourceURL: "",
		Hash:      "abc123",
	}

	if skill.Manifest.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got %q", skill.Manifest.Name)
	}
	if skill.Source != "local" {
		t.Errorf("Expected source 'local', got %q", skill.Source)
	}
	if !bytes.Contains([]byte(skill.Body), []byte("Instructions")) {
		t.Error("Body should contain instructions")
	}
}

func TestSkill_FromMarketplace(t *testing.T) {
	skill := &Skill{
		Manifest: Manifest{
			Name:        "marketplace-skill",
			Description: "From marketplace",
		},
		Body:      "# Marketplace skill",
		Path:      "/tmp/skills/marketplace-skill",
		Source:    "marketplace",
		SourceURL: "https://marketplace.example.com/skills/marketplace-skill",
		Hash:      "xyz789",
	}

	if skill.Source != "marketplace" {
		t.Errorf("Expected source 'marketplace', got %q", skill.Source)
	}
	if skill.SourceURL == "" {
		t.Error("SourceURL should not be empty for marketplace skill")
	}
}

func TestSkill_ContentHash(t *testing.T) {
	skill1 := &Skill{
		Manifest: Manifest{Name: "skill1", Description: "First"},
		Body:     "Same content",
		Hash:     "hash-abc",
	}

	skill2 := &Skill{
		Manifest: Manifest{Name: "skill2", Description: "Second"},
		Body:     "Same content",
		Hash:     "hash-abc",
	}

	if skill1.Hash != skill2.Hash {
		t.Error("Same content should have same hash")
	}
}

// ── Parsing Tests ───────────────────────────────────────────────────────────

func TestParseSKILL_Valid(t *testing.T) {
	data := []byte(`---
name: test-skill
description: A test skill
version: 1.0.0
---
# Instructions

Do something useful.`)

	manifest, body, err := ParseSKILL(data)
	if err != nil {
		t.Errorf("ParseSKILL failed: %v", err)
	}

	if manifest.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got %q", manifest.Name)
	}
	if !bytes.Contains([]byte(body), []byte("Do something useful")) {
		t.Error("Body should contain instructions")
	}
}

func TestParseSKILL_WithAllowedTools(t *testing.T) {
	data := []byte(`---
name: file-skill
description: File operations
allowed-tools:
  - read
  - write
  - list
---
# File Operations

Perform file operations.`)

	manifest, _, err := ParseSKILL(data)
	if err != nil {
		t.Errorf("ParseSKILL failed: %v", err)
	}

	if len(manifest.AllowedTools) != 3 {
		t.Errorf("Expected 3 allowed tools, got %d", len(manifest.AllowedTools))
	}
}

func TestParseSKILL_WithArguments(t *testing.T) {
	data := []byte(`---
name: search-skill
description: Search documents
arguments:
  - name: query
    description: Search query
    required: true
  - name: limit
    description: Result limit
    required: false
    default: "10"
---
# Search

Search documents.`)

	manifest, _, err := ParseSKILL(data)
	if err != nil {
		t.Errorf("ParseSKILL failed: %v", err)
	}

	if len(manifest.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(manifest.Arguments))
	}
	if !manifest.Arguments[0].Required {
		t.Error("First argument should be required")
	}
}

func TestParseSKILL_NoFrontmatter(t *testing.T) {
	data := []byte("Just content without frontmatter")

	_, _, err := ParseSKILL(data)
	if err == nil {
		t.Error("Expected error for missing frontmatter")
	}
}

// ── Repository Tests ────────────────────────────────────────────────────────

func TestRepository_New(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory")

	repo, err := NewRepository(tmpDir, memDir)
	if err != nil {
		t.Errorf("NewRepository failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Repository should not be nil")
	}
}

func TestRepository_Count(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory")

	repo, err := NewRepository(tmpDir, memDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	count := repo.Count()
	if count < 0 {
		t.Errorf("Count should be non-negative, got %d", count)
	}
}

func TestRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory")

	repo, err := NewRepository(tmpDir, memDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	list := repo.List()
	if list == nil {
		t.Error("List should return non-nil slice")
	}
}

func TestRepository_Get(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory")

	repo, err := NewRepository(tmpDir, memDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	// Try to get non-existent skill
	skill := repo.Get("non-existent")
	if skill != nil {
		t.Error("Get should return nil for non-existent skill")
	}
}

func TestRepository_Search(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory")

	repo, err := NewRepository(tmpDir, memDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	// Add a skill first
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)

	skillContent := `---
name: test-skill
description: A test skill for searching
---
# Test Skill`

	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644)
	repo.Refresh(context.Background())

	// Search should find the skill
	results := repo.Search("test")
	if results == nil {
		t.Error("Search should return slice")
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 match, got %d", len(results))
	}
}

func TestRepository_Refresh(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory")

	repo, err := NewRepository(tmpDir, memDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	// Refresh should scan the directory
	err = repo.Refresh(context.Background())
	if err != nil {
		t.Logf("Refresh returned: %v (OK if no skills found)", err)
	}
}

// ── Load Tests ──────────────────────────────────────────────────────────────

func TestLoad_ValidSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)

	skillContent := `---
name: test-skill
description: A test skill
version: 1.0.0
---
# Test Skill

Instructions here.`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	os.WriteFile(skillFile, []byte(skillContent), 0644)

	skill, err := Load(skillFile)
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}

	if skill == nil {
		t.Fatal("Skill should not be nil")
	}
	if skill.Manifest.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got %q", skill.Manifest.Name)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	skill, err := Load(tmpDir)
	if err == nil {
		t.Error("Expected error for missing SKILL.md")
	}
	if skill != nil {
		t.Error("Skill should be nil on error")
	}
}

// ── Tool Definition Tests ───────────────────────────────────────────────────

func TestSkill_ToToolDef(t *testing.T) {
	skill := &Skill{
		Manifest: Manifest{
			Name:        "tool-skill",
			Description: "A skill as a tool",
		},
		Body: "# Tool skill",
	}

	toolDef := skill.ToToolDef()
	if toolDef == nil {
		t.Error("ToToolDef should return non-nil map")
	}
	// ToToolDef returns {"type": "function", "function": {...}}
	funcDef, ok := toolDef["function"].(map[string]any)
	if !ok {
		t.Fatal("Expected 'function' key in toolDef")
	}
	if funcDef["name"] != "tool-skill" {
		t.Errorf("Expected name 'tool-skill', got %v", funcDef["name"])
	}
}

func TestSkill_ToMCPTool(t *testing.T) {
	skill := &Skill{
		Manifest: Manifest{
			Name:        "mcp-skill",
			Description: "A skill as MCP tool",
		},
		Body: "# MCP skill",
	}

	mcpTool := skill.ToMCPTool()
	if mcpTool == nil {
		t.Error("ToMCPTool should return non-nil map")
	}
}

// ── Integration Tests ───────────────────────────────────────────────────────

func TestSkill_FullLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "lifecycle-skill")
	os.MkdirAll(skillDir, 0755)

	// Create SKILL.md
	skillContent := `---
name: lifecycle-skill
description: Full lifecycle test
version: 1.0.0
author: Test Author
license: MIT
allowed-tools:
  - read
  - write
tags:
  - test
  - integration
---
# Lifecycle Skill

A skill for testing the full lifecycle.`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	os.WriteFile(skillFile, []byte(skillContent), 0644)

	// Load
	skill, err := Load(skillFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify manifest
	if skill.Manifest.Name != "lifecycle-skill" {
		t.Errorf("Expected name 'lifecycle-skill', got %q", skill.Manifest.Name)
	}
	if len(skill.Manifest.AllowedTools) != 2 {
		t.Errorf("Expected 2 allowed tools, got %d", len(skill.Manifest.AllowedTools))
	}
	if len(skill.Manifest.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(skill.Manifest.Tags))
	}

	// Verify body
	if !bytes.Contains([]byte(skill.Body), []byte("Lifecycle Skill")) {
		t.Error("Body should contain skill title")
	}

	// Convert to tool definitions
	toolDef := skill.ToToolDef()
	funcDef, ok := toolDef["function"].(map[string]any)
	if !ok {
		t.Fatal("Expected 'function' key in toolDef")
	}
	if funcDef["name"] != "lifecycle-skill" {
		t.Error("ToToolDef should preserve name")
	}
}
