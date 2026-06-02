// Package skill — AgentForge skill system: SKILL.md loader, marketplace, and lifecycle.
//
// Skills are modular capability units that extend agents. AgentForge implements
// the SKILL.md standard (Anthropic spec, adopted by Claude Code, Codex CLI,
// OpenClaw, Copilot) with AgentForge-specific extensions:
//
//   - Skills auto-register as tools in the tool registry
//   - Skill `allowed-tools` maps to capability enforcement
//   - Skills can be WASM-sandboxed (Rust SDK plugins)
//   - Skills follow the MeMex memory model (markdown + git)
//   - SkillsMP marketplace for discovery and installation
//
// SKILL.md format (Anthropic spec):
//
//	---
//	name: skill-name              # required, must match folder name
//	description: What and when.    # required, trigger matching sentence(s)
//	allowed-tools: [read, write]  # optional, capability scoping
//	context: fork                 # optional, isolated subagent execution
//	argument-hint: "..."          # optional
//	---
//	# Skill instructions...
package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ── SKILL.md Frontmatter ────────────────────────────────────────────────────

// Manifest is the parsed YAML frontmatter of a SKILL.md file.
type Manifest struct {
	Name                   string              `yaml:"name"`
	Description            string              `yaml:"description"`
	WhenToUse              string              `yaml:"when_to_use,omitempty"`
	ArgumentHint           string              `yaml:"argument-hint,omitempty"`
	Arguments              []ManifestArgument  `yaml:"arguments,omitempty"`
	AllowedTools           []string            `yaml:"allowed-tools,omitempty"`
	Context                string              `yaml:"context,omitempty"`      // "fork" | ""
	Model                  string              `yaml:"model,omitempty"`
	Effort                 string              `yaml:"effort,omitempty"`       // "low" | "medium" | "high"
	DisableModelInvocation bool                `yaml:"disable-model-invocation,omitempty"`
	Hooks                  map[string]HookDef  `yaml:"hooks,omitempty"`
	Tags                   []string            `yaml:"tags,omitempty"`         // AgentForge extension
	CapabilityRequired     []string            `yaml:"capability-required,omitempty"` // AgentForge extension
	Version                string              `yaml:"version,omitempty"`
	Author                 string              `yaml:"author,omitempty"`
	License                string              `yaml:"license,omitempty"`
}

// ManifestArgument describes a structured argument the skill accepts.
type ManifestArgument struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
}

// HookDef describes an event-driven hook.
type HookDef struct {
	Script string `yaml:"script,omitempty"`
	On     string `yaml:"on,omitempty"`
}

// Skill is a loaded skill — parsed manifest + raw markdown body.
type Skill struct {
	Manifest  Manifest
	Body      string   // markdown instruction body
	Path      string   // absolute directory path
	Source    string   // "local" | "marketplace" | "git"
	SourceURL string   // git URL if from marketplace
	Hash      string   // SHA-256 content hash for integrity
}

// ── Repository ──────────────────────────────────────────────────────────────

// Repository loads, caches, and queries skills from the filesystem.
type Repository struct {
	root      string          // ~/.agentforge/skills
	mu        sync.RWMutex
	skills    map[string]*Skill // name → skill
	memPath   string            // MeMex memory root for git tracking
}

// NewRepository creates/opens a skill repository at root.
func NewRepository(root, memPath string) (*Repository, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("skill repo: mkdir %s: %w", root, err)
	}

	r := &Repository{
		root:    root,
		skills:  make(map[string]*Skill),
		memPath: memPath,
	}

	// Load existing skills on init
	if err := r.Refresh(context.Background()); err != nil {
		return nil, fmt.Errorf("skill repo: refresh: %w", err)
	}

	return r, nil
}

// Refresh rescans the skills directory.
func (r *Repository) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills = make(map[string]*Skill)

	return filepath.Walk(r.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if path == r.root {
			return nil
		}

		skillPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			return nil // not a skill directory
		}

		skill, err := Load(skillPath)
		if err != nil {
			// Log but don't fail — skip malformed skills
			fmt.Fprintf(os.Stderr, "skill: skip %s: %v\n", path, err)
			return nil
		}

		r.skills[skill.Manifest.Name] = skill
		return filepath.SkipDir // don't recurse into skill dirs
	})
}

// Install copies a skill from sourcePath into the repository.
func (r *Repository) Install(ctx context.Context, sourcePath string) (*Skill, error) {
	skill, err := Load(filepath.Join(sourcePath, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}

	targetDir := filepath.Join(r.root, skill.Manifest.Name)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("install: mkdir %s: %w", targetDir, err)
	}

	// Copy entire directory
	if err := copyDir(sourcePath, targetDir); err != nil {
		return nil, fmt.Errorf("install: copy: %w", err)
	}

	skill.Path = targetDir
	skill.Source = "local"

	r.mu.Lock()
	r.skills[skill.Manifest.Name] = skill
	r.mu.Unlock()

	return skill, nil
}

// Get returns a skill by name, or nil.
func (r *Repository) Get(name string) *Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[name]
}

// List returns all installed skill names.
func (r *Repository) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for n := range r.skills {
		names = append(names, n)
	}
	return names
}

// Search finds skills whose description matches a query.
// This is the description-based auto-activation mechanism.
func (r *Repository) Search(query string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(query)
	var matches []*Skill
	for _, s := range r.skills {
		desc := strings.ToLower(s.Manifest.Description)
		when := strings.ToLower(s.Manifest.WhenToUse)
		if strings.Contains(desc, lower) || strings.Contains(when, lower) {
			matches = append(matches, s)
		}
	}
	return matches
}

// Count returns total number of installed skills.
func (r *Repository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// ── Loader ──────────────────────────────────────────────────────────────────

// Load reads and parses a SKILL.md file from path.
// Returns the parsed skill with frontmatter and body.
func Load(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skill: read %s: %w", path, err)
	}

	manifest, body, err := ParseSKILL(data)
	if err != nil {
		return nil, fmt.Errorf("skill: parse %s: %w", path, err)
	}

	// Validate required fields
	if manifest.Name == "" {
		return nil, fmt.Errorf("skill: missing required field 'name' in frontmatter")
	}
	if manifest.Description == "" {
		return nil, fmt.Errorf("skill: missing required field 'description' in frontmatter")
	}

	// Validate name matches directory (warning only in logs)
	dirName := filepath.Base(filepath.Dir(path))
	if manifest.Name != dirName {
		fmt.Fprintf(os.Stderr, "skill: warning: skill name %q doesn't match directory %q\n", manifest.Name, dirName)
	}

	return &Skill{
		Manifest: manifest,
		Body:     body,
		Path:     filepath.Dir(path),
		Source:   "local",
	}, nil
}

// ParseSKILL extracts YAML frontmatter and markdown body from SKILL.md content.
func ParseSKILL(data []byte) (Manifest, string, error) {
	content := string(data)

	// Strip leading whitespace
	content = strings.TrimSpace(content)

	// Must start with ---
	if !strings.HasPrefix(content, "---") {
		return Manifest{}, content, fmt.Errorf("skill: missing YAML frontmatter (must start with ---)")
	}

	// Find closing ---
	content = content[3:] // strip opening ---
	endIdx := strings.Index(content, "\n---")
	if endIdx == -1 {
		return Manifest{}, content, fmt.Errorf("skill: unclosed YAML frontmatter (missing closing ---)")
	}

	yamlBlock := content[:endIdx]
	markdownBody := strings.TrimSpace(content[endIdx+4:]) // strip \n---

	var manifest Manifest
	if err := yaml.Unmarshal([]byte(yamlBlock), &manifest); err != nil {
		return Manifest{}, markdownBody, fmt.Errorf("skill: YAML parse error: %w", err)
	}

	return manifest, markdownBody, nil
}

// ── Marketplace ─────────────────────────────────────────────────────────────

// MarketplaceEntry is a skill listed in a SkillsMP-compatible marketplace.
type MarketplaceEntry struct {
	Name        string            `json:"name"`
	Owner       MarketplaceOwner  `json:"owner"`
	Metadata    MarketplaceMeta   `json:"metadata"`
	Plugins     []MarketplacePlugin `json:"plugins"`
}

// MarketplaceOwner identifies the marketplace author.
type MarketplaceOwner struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// MarketplaceMeta is marketplace-level metadata.
type MarketplaceMeta struct {
	Description string `json:"description"`
	Version     string `json:"version"`
}

// MarketplacePlugin is a skill entry in the marketplace catalog.
type MarketplacePlugin struct {
	Name        string `json:"name"`
	Source      PluginSource `json:"source"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Keywords    []string `json:"keywords"`
	Strict      bool   `json:"strict"`
}

// PluginSource describes where the plugin is fetched from.
type PluginSource struct {
	Source string `json:"source"` // "url" | "github"
	URL    string `json:"url"`
}

// ── Skill → Tool Conversion ─────────────────────────────────────────────────

// ToToolDef converts a skill's frontmatter to a tool definition
// compatible with the LLM tool-calling interface.
func (s *Skill) ToToolDef() map[string]any {
	m := s.Manifest

	params := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}

	for _, arg := range m.Arguments {
		props := params["properties"].(map[string]any)
		props[arg.Name] = map[string]any{
			"type":        "string",
			"description": arg.Description,
		}
		if arg.Required {
			req := params["required"].([]string)
			params["required"] = append(req, arg.Name)
		}
	}

	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        m.Name,
			"description": m.Description,
			"parameters":  params,
		},
	}
}

// ToMCPTool converts a skill to MCP-compatible tool definition.
func (s *Skill) ToMCPTool() map[string]any {
	return map[string]any{
		"name":        s.Manifest.Name,
		"description": s.Manifest.Description,
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// Compile-time check
var _ = yaml.Unmarshal