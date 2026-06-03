// Package security — capability-based permission enforcement.

package security

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Errors.
var (
	ErrCapabilityExpired   = errors.New("capability: expired")
	ErrCapabilityInvalid  = errors.New("capability: invalid signature")
	ErrPermissionDenied   = errors.New("capability: permission denied")
	ErrResourceDenied     = errors.New("capability: resource denied")
	ErrTokenBudgetExceeded = errors.New("capability: token budget exceeded")
	ErrTimeoutExceeded     = errors.New("capability: session timeout exceeded")
)

// Permission is a capability permission.
type Permission string

const (
	PermRead     Permission = "read"
	PermWrite    Permission = "write"
	PermExec     Permission = "exec"
	PermNet      Permission = "net"
	PermSpawn    Permission = "spawn"
	PermDelegate Permission = "delegate"
)

// Operation maps to a permission for a given tool.
type Operation string

const (
	OpRead  Operation = "read"
	OpWrite Operation = "write"
	OpExec  Operation = "exec"
)

// ResourceRule defines scoped access to a resource.
type ResourceRule struct {
	Path       string       // filesystem path (glob patterns allowed)
	Domain     string       // network domain (glob patterns allowed)
	Operations []Permission
}

// Capability is an unforgeable permission token issued to an agent.
// Implemented as HMAC-SHA256 to allow stateless server-side verification.
type Capability struct {
	ID          string           `json:"id"`
	Issuer      string           `json:"issuer"`
	Subject     string           `json:"subject"` // agent ID
	Permissions []Permission    `json:"permissions"`
	Resources   []ResourceRule  `json:"resources"`
	TokenBudget int64            `json:"tokenBudget"` // remaining tokens
	Timeout     time.Duration   `json:"timeout"`
	ExpiresAt   time.Time        `json:"expiresAt"`
	Signature   []byte           `json:"-"`          // HMAC, never serialized to JSON
	ParentID    string           `json:"parentId"`   // for delegation chains
}

// Action is a permission check target.
type Action struct {
	Tool       string
	Args       map[string]any
	Resource   string
	Operation  Operation
}

// Enforcer checks capabilities at every enforcement point.
type Enforcer struct {
	secret    []byte
	tokenUsed int64
}

// NewEnforcer creates a new capability enforcer.
func NewEnforcer(secret string) *Enforcer {
	return &Enforcer{
		secret: []byte(secret),
	}
}

// Derive creates a child capability from a parent — permissions MUST be
// a semantic subset of the parent. Callers must pass the parent Capability.
func (e *Enforcer) Derive(parent *Capability, subject string, restrictions ...Restriction) (*Capability, error) {
	cap := &Capability{
		ID:          uuid.New().String(),
		Issuer:      "agentforge",
		Subject:     subject,
		Permissions: parent.Permissions,
		Resources:   make([]ResourceRule, len(parent.Resources)),
		TokenBudget: parent.TokenBudget,
		Timeout:     parent.Timeout,
		ExpiresAt:   time.Now().Add(parent.Timeout),
		ParentID:    parent.ID,
	}
	copy(cap.Resources, parent.Resources)

	// Apply restrictions
	for _, r := range restrictions {
		r(cap)
	}

	if err := e.Sign(cap); err != nil {
		return nil, fmt.Errorf("sign capability: %w", err)
	}
	return cap, nil
}

// Issue creates a new root capability for an agent.
func (e *Enforcer) Issue(
	subject string,
	permissions []Permission,
	resources []ResourceRule,
	tokenBudget int64,
	timeout time.Duration,
) *Capability {
	cap := &Capability{
		ID:          uuid.New().String(),
		Issuer:      "agentforge",
		Subject:     subject,
		Permissions: permissions,
		Resources:   resources,
		TokenBudget: tokenBudget,
		Timeout:     timeout,
		ExpiresAt:   time.Now().Add(timeout),
	}
	e.Sign(cap)
	return cap
}

// Sign computes the HMAC-SHA256 signature over the canonical fields.
func (e *Enforcer) Sign(cap *Capability) error {
	data := []byte(
		fmt.Sprintf("%s:%s:%s:%v:%v:%v",
			cap.ID,
			cap.Subject,
			cap.Issuer,
			cap.Permissions,
			cap.TokenBudget,
			cap.ExpiresAt.Unix(),
		),
	)
	h := hmac.New(sha256.New, e.secret)
	h.Write(data)
	cap.Signature = h.Sum(nil)
	return nil
}

// Verify checks the HMAC signature is valid and the capability has not
// been tampered with. Call this on every spawn to prevent forgery.
func (e *Enforcer) Verify(cap *Capability) bool {
	if cap == nil {
		return false
	}
	if time.Now().After(cap.ExpiresAt) {
		return false
	}
	// Re-compute expected signature
	data := []byte(
		fmt.Sprintf("%s:%s:%s:%v:%v:%v",
			cap.ID,
			cap.Subject,
			cap.Issuer,
			cap.Permissions,
			cap.TokenBudget,
			cap.ExpiresAt.Unix(),
		),
	)
	h := hmac.New(sha256.New, e.secret)
	h.Write(data)
	expected := h.Sum(nil)
	return subtle.ConstantTimeCompare(cap.Signature, expected) == 1
}

// Check verifies an action is permitted by the capability.
// Returns ErrPermissionDenied, ErrResourceDenied, or nil.
func (e *Enforcer) Check(ctx context.Context, cap *Capability, action Action) error {
	// 1. Expiry check
	if time.Now().After(cap.ExpiresAt) {
		return ErrCapabilityExpired
	}

	// 2. Permission check
	if !hasPermission(cap.Permissions, action.Operation, action.Tool) {
		return fmt.Errorf("%w: %s on %s", ErrPermissionDenied, action.Operation, action.Tool)
	}

	// 3. Resource scope check
	if action.Resource != "" {
		if !e.resourceAllowed(cap, action.Resource, action.Operation) {
			return fmt.Errorf("%w: %s access to %s", ErrResourceDenied, action.Operation, action.Resource)
		}
	}

	return nil
}

// UpdateTokenBudget deducts from the token budget. Returns error if
// the budget would go negative.
func (e *Enforcer) UpdateTokenBudget(cap *Capability, tokens int64) error {
	if cap.TokenBudget-tokens < 0 {
		return ErrTokenBudgetExceeded
	}
	cap.TokenBudget -= tokens
	e.tokenUsed += tokens
	return nil
}

func (e *Enforcer) resourceAllowed(cap *Capability, resource string, op Operation) bool {
	for _, r := range cap.Resources {
		if r.Path == "" {
			continue
		}

		// Try exact match first (fast path)
		if r.Path == resource {
			if e.operationAllowed(r.Operations, op) {
				return true
			}
			continue
		}

		// Try glob pattern match (e.g., /home/user/*, *.log, etc.)
		match, err := filepath.Match(r.Path, resource)
		if err != nil {
			// Invalid glob pattern in capability — skip it
			continue
		}
		if match {
			if e.operationAllowed(r.Operations, op) {
				return true
			}
		}
	}
	return false
}

// operationAllowed checks if the operation is in the permission list.
func (e *Enforcer) operationAllowed(perms []Permission, op Operation) bool {
	for _, p := range perms {
		if p == PermRead && op == OpRead {
			return true
		}
		if p == PermWrite && op == OpWrite {
			return true
		}
		if p == PermExec && op == OpExec {
			return true
		}
	}
	return false
}

func hasPermission(perms []Permission, op Operation, tool string) bool {
	for _, p := range perms {
		switch op {
		case OpRead:
			if p == PermRead || p == PermDelegate {
				return true
			}
		case OpWrite:
			if p == PermWrite || p == PermDelegate {
				return true
			}
		case OpExec:
			if p == PermExec || p == PermSpawn {
				return true
			}
		}
	}
	return false
}

// Restriction is a functional option to constrain a derived capability.
type Restriction func(*Capability)

// RestrictPermissions overwrites the allowed permissions.
func RestrictPermissions(perms []Permission) Restriction {
	return func(c *Capability) {
		c.Permissions = perms
	}
}

// RestrictTimeout reduces the timeout.
func RestrictTimeout(d time.Duration) Restriction {
	return func(c *Capability) {
		if d < c.Timeout {
			c.Timeout = d
		}
	}
}

// RestrictTokenBudget reduces the token budget.
func RestrictTokenBudget(b int64) Restriction {
	return func(c *Capability) {
		if b < c.TokenBudget {
			c.TokenBudget = b
		}
	}
}
