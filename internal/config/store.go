// Package config — configuration management for AgentForge.
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// PersistedStore enables runtime config reads + writes to ~/.agentforge/agentforge.yaml.
type PersistedStore struct {
	mu   sync.RWMutex
	cfg  *Config
	path string
}

// NewStore loads config and tracks the file path for writes.
func NewStore(cfg *Config) *PersistedStore {
	return &PersistedStore{cfg: cfg, path: ConfigFilePath()}
}

// Cfg returns the current config (read-safe).
func (s *PersistedStore) Cfg() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Update applies a patch of string values and persists to disk.
// Keys use dot notation matching the YAML structure.
// Values that are JSON arrays or objects are parsed and merged into the YAML tree.
func (s *PersistedStore) Update(patch map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	for key, val := range patch {
		parts := strings.Split(key, ".")
		root := &doc
		if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
			root = doc.Content[0]
		}

		// Navigate/create path to the leaf
		for i := 0; i < len(parts)-1; i++ {
			ensureKeyExists(root, parts[i])
			root = getOrCreateChild(root, parts[i])
		}

		// Now set the leaf
		setLeafValue(root, parts[len(parts)-1], val)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(s.path, out, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	newCfg, err := Load(s.path)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	s.cfg = newCfg
	return nil
}

// ensureKeyExists guarantees key exists in the mapping node.
func ensureKeyExists(parent *yaml.Node, key string) {
	if parent.Kind != yaml.MappingNode {
		// Convert scalar/sequence to mapping
		parent.Kind = yaml.MappingNode
		parent.Tag = "!!map"
		parent.Content = nil
	}
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			return // already exists
		}
	}
	// Key doesn't exist — create it with an empty mapping
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
	)
}

// getOrCreateChild finds or creates the value node for a given key in mapping.
func getOrCreateChild(parent *yaml.Node, key string) *yaml.Node {
	if parent.Kind != yaml.MappingNode {
		parent.Kind = yaml.MappingNode
		parent.Tag = "!!map"
		parent.Content = nil
	}
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	// Create it
	child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		child,
	)
	return child
}

// setLeafValue sets the final value for a key in a mapping node.
func setLeafValue(parent *yaml.Node, key, val string) {
	trimmed := strings.TrimSpace(val)

	// Try JSON array/object — yaml.v3 parses JSON natively
	if (strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{")) &&
		(strings.Contains(trimmed, ":") || strings.Contains(trimmed, ",")) {

		var parsed yaml.Node
		if err := yaml.Unmarshal([]byte(val), &parsed); err == nil {
			if parsed.Kind == yaml.DocumentNode && len(parsed.Content) > 0 {
				parsed = *parsed.Content[0]
			}
			setChild(parent, key, &parsed)
			return
		}
	}

	// Scalar
	setChild(parent, key, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: val,
		Tag:   scalarTag(val),
	})
}

// setChild replaces or appends a key→value entry in a mapping node.
func setChild(parent *yaml.Node, key string, child *yaml.Node) {
	if parent.Kind != yaml.MappingNode {
		parent.Kind = yaml.MappingNode
		parent.Tag = "!!map"
		parent.Content = nil
	}
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = child
			return
		}
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		child,
	)
}

func scalarTag(v string) string {
	switch v {
	case "true", "false":
		return "!!bool"
	default:
		return "!!str"
	}
}