package security

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ── Glob Pattern Matching Tests ──────────────────────────────────────────────

func TestResourceAllowed_ExactMatch(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent1",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "/home/user/secret.txt",
				Operations:  []Permission{PermRead},
			},
		},
	}

	// Exact match should work
	if !enforcer.resourceAllowed(cap, "/home/user/secret.txt", OpRead) {
		t.Fatal("Expected exact match to allow access")
	}

	// Different path should not match
	if enforcer.resourceAllowed(cap, "/home/user/other.txt", OpRead) {
		t.Fatal("Expected different path to deny access")
	}

	t.Log("Exact match verification passed")
}

func TestResourceAllowed_GlobWildcard(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent2",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "/home/user/*",
				Operations:  []Permission{PermRead},
			},
		},
	}

	// Glob pattern should match files in directory
	tests := []struct {
		resource string
		allowed  bool
		desc     string
	}{
		{"/home/user/secret.txt", true, "matches /home/user/secret.txt"},
		{"/home/user/readme.md", true, "matches /home/user/readme.md"},
		{"/home/user/nested/file.txt", false, "does not match nested path"},
		{"/home/other/file.txt", false, "does not match different directory"},
	}

	for _, tt := range tests {
		allowed := enforcer.resourceAllowed(cap, tt.resource, OpRead)
		if allowed != tt.allowed {
			t.Errorf("Pattern /home/user/* with %s: got %v, want %v", tt.desc, allowed, tt.allowed)
		}
	}

	t.Log("Glob wildcard verification passed")
}

func TestResourceAllowed_GlobExtensionFilter(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent3",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "/var/log/*.log",
				Operations:  []Permission{PermRead},
			},
		},
	}

	tests := []struct {
		resource string
		allowed  bool
		desc     string
	}{
		{"/var/log/app.log", true, "matches .log extension"},
		{"/var/log/2026-06-03.log", true, "matches dated .log file"},
		{"/var/log/app.txt", false, "does not match .txt extension"},
		{"/var/log/readme", false, "does not match file with no extension"},
		{"/var/log/subdir/app.log", false, "does not match nested .log"},
	}

	for _, tt := range tests {
		allowed := enforcer.resourceAllowed(cap, tt.resource, OpRead)
		if allowed != tt.allowed {
			t.Errorf("Pattern /var/log/*.log with %s: got %v, want %v", tt.desc, allowed, tt.allowed)
		}
	}

	t.Log("Glob extension filter verification passed")
}

func TestResourceAllowed_GlobQuestionMark(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent4",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "/etc/pass?",
				Operations:  []Permission{PermRead},
			},
		},
	}

	tests := []struct {
		resource string
		allowed  bool
		desc     string
	}{
		{"/etc/passw", true, "matches single character 'w'"},
		{"/etc/passx", true, "matches single character 'x'"},
		{"/etc/pass1", true, "matches single digit"},
		{"/etc/passwd", false, "does not match 'd' (pattern is /etc/pass?, matching 5 chars)"},
		{"/etc/passwdd", false, "does not match multiple extra characters"},
		{"/etc/pas", false, "does not match missing character"},
	}

	for _, tt := range tests {
		allowed := enforcer.resourceAllowed(cap, tt.resource, OpRead)
		if allowed != tt.allowed {
			t.Errorf("Pattern /etc/pass? with %s: got %v, want %v", tt.desc, allowed, tt.allowed)
		}
	}

	t.Log("Glob question mark verification passed")
}

func TestResourceAllowed_GlobCharacterRange(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent5",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "/data/[a-c].txt",
				Operations:  []Permission{PermRead},
			},
		},
	}

	tests := []struct {
		resource string
		allowed  bool
		desc     string
	}{
		{"/data/a.txt", true, "matches a in range [a-c]"},
		{"/data/b.txt", true, "matches b in range [a-c]"},
		{"/data/c.txt", true, "matches c in range [a-c]"},
		{"/data/d.txt", false, "does not match d outside range [a-c]"},
		{"/data/z.txt", false, "does not match z outside range [a-c]"},
	}

	for _, tt := range tests {
		allowed := enforcer.resourceAllowed(cap, tt.resource, OpRead)
		if allowed != tt.allowed {
			t.Errorf("Pattern /data/[a-c].txt with %s: got %v, want %v", tt.desc, allowed, tt.allowed)
		}
	}

	t.Log("Glob character range verification passed")
}

func TestResourceAllowed_InvalidGlobPattern(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent6",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "[invalid",  // Missing closing bracket
				Operations:  []Permission{PermRead},
			},
		},
	}

	// Invalid pattern should not crash, just fail to match
	// This tests graceful degradation
	if enforcer.resourceAllowed(cap, "/[invalid", OpRead) {
		t.Error("Expected invalid pattern to not match anything")
	}

	if enforcer.resourceAllowed(cap, "anything", OpRead) {
		t.Error("Expected invalid pattern to not match any path")
	}

	t.Log("Invalid pattern gracefully handled (no panic)")
}

func TestResourceAllowed_OperationMatching(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent7",
		Permissions: []Permission{PermRead, PermWrite},
		Resources: []ResourceRule{
			{
				Path:        "/home/user/*",
				Operations:  []Permission{PermRead, PermWrite},
			},
		},
	}

	path := "/home/user/file.txt"

	// Read should be allowed
	if !enforcer.resourceAllowed(cap, path, OpRead) {
		t.Error("Expected read operation to be allowed")
	}

	// Write should be allowed
	if !enforcer.resourceAllowed(cap, path, OpWrite) {
		t.Error("Expected write operation to be allowed")
	}

	// Exec should not be allowed (not in Operations)
	if enforcer.resourceAllowed(cap, path, OpExec) {
		t.Error("Expected exec operation to be denied")
	}

	t.Log("Operation matching verification passed")
}

func TestResourceAllowed_MultipleRules(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent8",
		Permissions: []Permission{PermRead, PermWrite},
		Resources: []ResourceRule{
			{
				Path:        "/home/user/readonly/*",
				Operations:  []Permission{PermRead},
			},
			{
				Path:        "/home/user/readwrite/*",
				Operations:  []Permission{PermRead, PermWrite},
			},
		},
	}

	// Test readonly directory
	if !enforcer.resourceAllowed(cap, "/home/user/readonly/file.txt", OpRead) {
		t.Error("Expected read in readonly to be allowed")
	}
	if enforcer.resourceAllowed(cap, "/home/user/readonly/file.txt", OpWrite) {
		t.Error("Expected write in readonly to be denied")
	}

	// Test readwrite directory
	if !enforcer.resourceAllowed(cap, "/home/user/readwrite/file.txt", OpRead) {
		t.Error("Expected read in readwrite to be allowed")
	}
	if !enforcer.resourceAllowed(cap, "/home/user/readwrite/file.txt", OpWrite) {
		t.Error("Expected write in readwrite to be allowed")
	}

	t.Log("Multiple rules verification passed")
}

func TestResourceAllowed_EmptyPath(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent9",
		Permissions: []Permission{PermRead},
		Resources: []ResourceRule{
			{
				Path:        "",  // Empty path should be skipped
				Operations:  []Permission{PermRead},
			},
		},
	}

	// Empty path should not match anything
	if enforcer.resourceAllowed(cap, "any/path", OpRead) {
		t.Error("Expected empty path to not match")
	}

	t.Log("Empty path correctly skipped")
}

func TestResourceAllowed_NoResources(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	cap := &Capability{
		ID:          "test-cap",
		Subject:     "agent10",
		Permissions: []Permission{PermRead},
		Resources:   []ResourceRule{},  // No resources
	}

	// No resources means no access
	if enforcer.resourceAllowed(cap, "/any/path", OpRead) {
		t.Error("Expected no resources to deny access")
	}

	t.Log("No resources correctly denied")
}

// ── Capability Enforcer Integration Tests ────────────────────────────────────

func TestEnforcer_EvalWithGlobPatterns(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	// Create a capability with glob patterns
	cap := &Capability{
		ID:          "glob-cap",
		Issuer:      "test",
		Subject:     "agent-glob",
		Permissions: []Permission{PermRead, PermWrite},
		Resources: []ResourceRule{
			{
				Path:        "/home/*/docs/*",
				Operations:  []Permission{PermRead},
			},
			{
				Path:        "/tmp/*.tmp",
				Operations:  []Permission{PermWrite},
			},
		},
		Timeout:   30 * time.Second,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// Sign the capability (normally done by Issue, but we're testing directly)
	enforcer.Sign(cap)

	tests := []struct {
		action    Action
		allowed   bool
		desc      string
	}{
		{
			Action{Resource: "/home/alice/docs/readme.txt", Operation: OpRead},
			true,
			"read matches /home/*/docs/* pattern",
		},
		{
			Action{Resource: "/home/bob/docs/guide.md", Operation: OpRead},
			true,
			"read matches /home/*/docs/* pattern for different user",
		},
		{
			Action{Resource: "/home/alice/private/secret.txt", Operation: OpRead},
			false,
			"read does not match /home/*/docs/* pattern",
		},
		{
			Action{Resource: "/tmp/cache.tmp", Operation: OpWrite},
			true,
			"write matches /tmp/*.tmp pattern",
		},
		{
			Action{Resource: "/tmp/file.log", Operation: OpWrite},
			false,
			"write does not match /tmp/*.tmp pattern",
		},
	}

	for _, tt := range tests {
		err := enforcer.Check(context.Background(), cap, tt.action)
		allowed := (err == nil)
		if allowed != tt.allowed {
			t.Errorf("%s: got %v, want %v (err: %v)", tt.desc, allowed, tt.allowed, err)
		}
	}

	t.Log("Integration test with glob patterns passed")
}

// ── Helper function tests ────────────────────────────────────────────────────

func TestOperationAllowed_AllOperations(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	tests := []struct {
		perms     []Permission
		op        Operation
		allowed   bool
		desc      string
	}{
		{[]Permission{PermRead}, OpRead, true, "PermRead allows OpRead"},
		{[]Permission{PermRead}, OpWrite, false, "PermRead denies OpWrite"},
		{[]Permission{PermWrite}, OpWrite, true, "PermWrite allows OpWrite"},
		{[]Permission{PermExec}, OpExec, true, "PermExec allows OpExec"},
		{[]Permission{PermRead, PermWrite}, OpRead, true, "multiple perms allows OpRead"},
		{[]Permission{}, OpRead, false, "empty perms denies OpRead"},
	}

	for _, tt := range tests {
		allowed := enforcer.operationAllowed(tt.perms, tt.op)
		if allowed != tt.allowed {
			t.Errorf("%s: got %v, want %v", tt.desc, allowed, tt.allowed)
		}
	}

	t.Log("Operation matching verification passed")
}

// ── Capability Derivation Tests ──────────────────────────────────────────────

func TestDerive_ValidSubset(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	parentCap := &Capability{
		ID:          "parent-1",
		Issuer:      "test",
		Subject:     "parent-agent",
		Permissions: []Permission{PermRead, PermWrite, PermExec},
		Resources: []ResourceRule{
			{
				Path:        "/home/user/*",
				Operations:  []Permission{PermRead, PermWrite},
			},
		},
		TokenBudget: 100000,
		Timeout:     1 * time.Hour,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Derive with restrictions - reduce permissions
	derived, err := enforcer.Derive(parentCap, "child-agent",
		RestrictPermissions([]Permission{PermRead}), // Only read permission
	)

	if err != nil {
		t.Fatalf("Valid derivation failed: %v", err)
	}

	if len(derived.Permissions) != 1 || derived.Permissions[0] != PermRead {
		t.Error("Derived permission not properly restricted")
	}

	if derived.ParentID != parentCap.ID {
		t.Error("Parent ID not set correctly")
	}

	t.Log("Valid derivation accepted")
}

func TestDerive_RejectsInvalidPermissions(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	parentCap := &Capability{
		ID:          "parent-2",
		Issuer:      "test",
		Subject:     "parent-agent",
		Permissions: []Permission{PermRead, PermWrite}, // No exec
		Resources:   []ResourceRule{},
		TokenBudget: 100000,
		Timeout:     1 * time.Hour,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Try to derive with exec permission (not in parent)
	derived, err := enforcer.Derive(parentCap, "child-agent",
		RestrictPermissions([]Permission{PermRead, PermExec}), // PermExec not allowed!
	)

	if err == nil {
		t.Fatal("Should have rejected derivation with non-parent permission")
	}

	if derived != nil {
		t.Error("Should not return capability on error")
	}

	if !strings.Contains(err.Error(), "not in parent") {
		t.Errorf("Expected 'not in parent' error, got: %v", err)
	}

	t.Log("Invalid permissions correctly rejected")
}

func TestDerive_RejectsTokenBudgetIncrease(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	parentCap := &Capability{
		ID:          "parent-3",
		Issuer:      "test",
		Subject:     "parent-agent",
		Permissions: []Permission{PermRead},
		Resources:   []ResourceRule{},
		TokenBudget: 1000, // Limited budget
		Timeout:     1 * time.Hour,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Try to increase token budget
	derived, err := enforcer.Derive(parentCap, "child-agent",
		func(c *Capability) {
			c.TokenBudget = 5000 // Increase budget - not allowed!
		},
	)

	if err == nil {
		t.Fatal("Should have rejected token budget increase")
	}

	if derived != nil {
		t.Error("Should not return capability on error")
	}

	if !strings.Contains(err.Error(), "token budget") {
		t.Errorf("Expected 'token budget' error, got: %v", err)
	}

	t.Log("Token budget increase correctly rejected")
}

func TestDerive_RejectsTimeoutIncrease(t *testing.T) {
	enforcer := NewEnforcer("test-secret")

	parentCap := &Capability{
		ID:          "parent-4",
		Issuer:      "test",
		Subject:     "parent-agent",
		Permissions: []Permission{PermRead},
		Resources:   []ResourceRule{},
		TokenBudget: 10000,
		Timeout:     1 * time.Hour,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Try to increase timeout
	derived, err := enforcer.Derive(parentCap, "child-agent",
		func(c *Capability) {
			c.Timeout = 2 * time.Hour // Increase timeout - not allowed!
		},
	)

	if err == nil {
		t.Fatal("Should have rejected timeout increase")
	}

	if derived != nil {
		t.Error("Should not return capability on error")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected 'timeout' error, got: %v", err)
	}

	t.Log("Timeout increase correctly rejected")
}

func TestDerive_PrivilegeEscalationPrevented(t *testing.T) {
	// Test that a malicious caller can't escalate privileges through derivation
	enforcer := NewEnforcer("test-secret")

	parentCap := &Capability{
		ID:          "parent-5",
		Issuer:      "test",
		Subject:     "parent-agent",
		Permissions: []Permission{PermRead}, // Only read
		Resources:   []ResourceRule{},
		TokenBudget: 10000,
		Timeout:     1 * time.Hour,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Attempt to escalate: add PermExec, increase budget, increase timeout
	_, err := enforcer.Derive(parentCap, "evil-child",
		func(c *Capability) {
			c.Permissions = []Permission{PermRead, PermWrite, PermExec} // Escalate
			c.TokenBudget = 999999                                      // Increase
			c.Timeout = 10 * time.Hour                                  // Increase
		},
	)

	if err == nil {
		t.Fatal("Privilege escalation should have been blocked")
	}

	t.Logf("Privilege escalation blocked: %v", err)
}
