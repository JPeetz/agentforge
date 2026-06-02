// Package auth — JWT+RBAC authentication for AgentForge.
//
// Provides:
//   - HS256 JWT generation/validation using the CapabilitySecret from config
//   - Access tokens (15min) + Refresh tokens (7 days)
//   - RBAC permission model: admin, operator, viewer
//   - Claims carrying UserID, Role, AgentID
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ── Token types ──────────────────────────────────────────────────────────────

// TokenType distinguishes access vs refresh tokens.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// ── Roles ────────────────────────────────────────────────────────────────────

// Role defines the permission tier for a user or API key.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Valid checks the role is one of the defined constants.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	}
	return false
}

// ── Resources & actions ──────────────────────────────────────────────────────

// Resource is a protected entity in the AgentForge system.
type Resource string

const (
	ResAgents     Resource = "agents"
	ResPipelines  Resource = "pipelines"
	ResLogs       Resource = "logs"
	ResChannels   Resource = "channels"
	ResDashboard  Resource = "dashboard"
	ResConfig     Resource = "config"
	ResMemory     Resource = "memory"
	ResSkills     Resource = "skills"
	ResTools      Resource = "tools"
	ResMCP        Resource = "mcp"
	ResAuth       Resource = "auth"
	ResSecurity   Resource = "security"
	ResSession    Resource = "session"
)

// Action is the operation being requested.
type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionAdmin  Action = "admin"
)

// ── RBAC permission map ─────────────────────────────────────────────────────

// rolePerms maps each role to its allowed (resource, action) combinations.
var rolePerms = map[Role]map[Resource][]Action{
	RoleAdmin: {
		// admin can do everything everywhere
		ResAgents:    {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
		ResPipelines: {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
		ResLogs:      {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
		ResChannels:  {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
		ResDashboard: {ActionRead, ActionWrite, ActionAdmin},
		ResConfig:    {ActionRead, ActionWrite, ActionAdmin},
		ResMemory:    {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
		ResSkills:    {ActionRead, ActionWrite, ActionAdmin},
		ResTools:     {ActionRead, ActionWrite, ActionAdmin},
		ResMCP:       {ActionRead, ActionWrite, ActionAdmin},
		ResAuth:      {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
		ResSecurity:  {ActionRead, ActionWrite, ActionAdmin},
		ResSession:   {ActionRead, ActionWrite, ActionDelete, ActionAdmin},
	},
	RoleOperator: {
		ResAgents:    {ActionRead, ActionWrite},
		ResPipelines: {ActionRead, ActionWrite},
		ResLogs:      {ActionRead},
		ResChannels:  {ActionRead, ActionWrite},
		ResDashboard: {ActionRead},
		ResMemory:    {ActionRead},
		ResSkills:    {ActionRead},
		ResTools:     {ActionRead},
		ResMCP:       {ActionRead},
		ResSession:   {ActionRead},
	},
	RoleViewer: {
		ResDashboard: {ActionRead},
		ResLogs:      {ActionRead},
		ResAgents:    {ActionRead},
		ResPipelines: {ActionRead},
		ResMemory:    {ActionRead},
	},
}

// HasPermission checks whether the given role can perform the action on a resource.
func HasPermission(role Role, resource Resource, action Action) bool {
	actions, ok := rolePerms[role][resource]
	if !ok {
		return false
	}
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

// ── JWT Claims ───────────────────────────────────────────────────────────────

// Claims is the custom JWT payload.
type Claims struct {
	jwt.RegisteredClaims
	UserID   string    `json:"uid"`
	Role     Role      `json:"role"`
	AgentID  string    `json:"aid,omitempty"` // optional: scoped to a specific agent
	TokenType TokenType `json:"typ"`
}

// ── Token manager ────────────────────────────────────────────────────────────

// Manager holds signing config and generates/validates JWTs.
type Manager struct {
	secret     []byte
	method     jwt.SigningMethod
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// ManagerConfig configures the JWT token manager.
type ManagerConfig struct {
	Secret        []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Issuer        string
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig(secret []byte) ManagerConfig {
	return ManagerConfig{
		Secret:     secret,
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		Issuer:     "agentforge",
	}
}

// NewManager creates a JWT token manager.
func NewManager(cfg ManagerConfig) *Manager {
	if len(cfg.Secret) == 0 {
		// Use a warning-only fallback — not secure for production
		cfg.Secret = []byte("agentforge-insecure-fallback-secret-change-me")
	}
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "agentforge"
	}
	return &Manager{
		secret:     cfg.Secret,
		method:     jwt.SigningMethodHS256,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		issuer:     cfg.Issuer,
	}
}

// ── Token generation ─────────────────────────────────────────────────────────

// GenerateTokenPair creates an access + refresh token pair for a user.
func (m *Manager) GenerateTokenPair(userID string, role Role) (accessToken, refreshToken string, err error) {
	if !role.Valid() {
		return "", "", fmt.Errorf("auth: invalid role %q", role)
	}
	now := time.Now()

	access, err := m.sign(Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			ID:        generateTokenID("ac"),
		},
		UserID:    userID,
		Role:      role,
		TokenType: AccessToken,
	})
	if err != nil {
		return "", "", fmt.Errorf("auth: sign access: %w", err)
	}

	refresh, err := m.sign(Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
			ID:        generateTokenID("rf"),
		},
		UserID:    userID,
		Role:      role,
		TokenType: RefreshToken,
	})
	if err != nil {
		return "", "", fmt.Errorf("auth: sign refresh: %w", err)
	}

	return access, refresh, nil
}

// RefreshAccessToken validates the refresh token and issues a new pair.
func (m *Manager) RefreshAccessToken(refreshTokenString string) (accessToken, refreshToken string, err error) {
	claims, err := m.ValidateToken(refreshTokenString)
	if err != nil {
		return "", "", fmt.Errorf("auth: refresh: %w", err)
	}
	if claims.TokenType != RefreshToken {
		return "", "", errors.New("auth: refresh: token is not a refresh token")
	}
	return m.GenerateTokenPair(claims.UserID, claims.Role)
}

// sign builds and signs a JWT.
func (m *Manager) sign(claims Claims) (string, error) {
	token := jwt.NewWithClaims(m.method, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", err
	}
	return signed, nil
}

// ── Token validation ─────────────────────────────────────────────────────────

// ValidateToken parses and validates a JWT, returning the claims.
func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("auth: token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("auth: token invalid")
	}

	return claims, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// generateTokenID creates a short unique token identifier.
var tokenIDCounter int64
var tokenIDBase = time.Now().UnixNano()

func generateTokenID(prefix string) string {
	// Simple monotonically increasing ID — collisions impossible within a process.
	tokenIDCounter++
	return fmt.Sprintf("%s_%d", prefix, tokenIDBase+tokenIDCounter)
}

// ── Compile-time interface checks ────────────────────────────────────────────

// TokenValidator is satisfied by *Manager.
type TokenValidator interface {
	ValidateToken(tokenString string) (*Claims, error)
}

// TokenGenerator is satisfied by *Manager.
type TokenGenerator interface {
	GenerateTokenPair(userID string, role Role) (string, string, error)
}

var _ TokenValidator = (*Manager)(nil)
var _ TokenGenerator = (*Manager)(nil)