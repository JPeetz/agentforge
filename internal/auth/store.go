// Package auth — user and API key storage for AgentForge.
//
// Provides an in-memory user store with a default admin user seeded from
// the config's CapabilitySecret. Supports API key generation scoped per-agent.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ── User ─────────────────────────────────────────────────────────────────────

// User represents a registered dashboard/API user.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // never serialized
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ── API Key ───────────────────────────────────────────────────────────────────

// APIKey represents a scoped access key (typically per-agent).
type APIKey struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	KeyHash   string    `json:"-"` // sha256 of the key, never serialized
	Prefix    string    `json:"prefix"` // first 8 chars for display
	Scopes    []string  `json:"scopes"` // agent IDs or "*"
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	LastUsed  time.Time `json:"lastUsed,omitempty"`
}

// ── Store ────────────────────────────────────────────────────────────────────

// Store holds users and API keys in memory.
type Store struct {
	mu     sync.RWMutex
	users  map[string]*User   // userID -> User
	byName map[string]*User   // username -> User
	keys   map[string]*APIKey // keyHash -> APIKey
}

// NewStore creates an in-memory user and API key store, seeding the default admin user.
// The admin password is derived from the provided capabilitySecret.
func NewStore(capabilitySecret string) *Store {
	s := &Store{
		users:  make(map[string]*User),
		byName: make(map[string]*User),
		keys:   make(map[string]*APIKey),
	}

	// Seed default admin user
	adminHash := hashPassword(capabilitySecret)
	admin := &User{
		ID:           "admin-001",
		Username:     "admin",
		PasswordHash: adminHash,
		Role:         RoleAdmin,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	s.users[admin.ID] = admin
	s.byName[admin.Username] = admin

	return s
}

// ── User CRUD ────────────────────────────────────────────────────────────────

// AuthenticateUser verifies username and password, returning the user if valid.
func (s *Store) AuthenticateUser(username, password string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.byName[username]
	if !ok {
		return nil, fmt.Errorf("auth: user %q not found", username)
	}

	if !verifyPassword(user.PasswordHash, password) {
		return nil, fmt.Errorf("auth: invalid password")
	}

	return user, nil
}

// GetUser returns a user by ID.
func (s *Store) GetUser(userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return nil, fmt.Errorf("auth: user %q not found", userID)
	}
	return user, nil
}

// GetUserByName returns a user by username.
func (s *Store) GetUserByName(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.byName[username]
	if !ok {
		return nil, fmt.Errorf("auth: user %q not found", username)
	}
	return user, nil
}

// CreateUser adds a new user to the store.
func (s *Store) CreateUser(username, password string, role Role) (*User, error) {
	if !role.Valid() {
		return nil, fmt.Errorf("auth: invalid role %q", role)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.byName[username]; ok {
		return nil, fmt.Errorf("auth: user %q already exists", username)
	}

	now := time.Now()
	user := &User{
		ID:           fmt.Sprintf("user-%d", now.UnixNano()),
		Username:     username,
		PasswordHash: hashPassword(password),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.users[user.ID] = user
	s.byName[user.Username] = user
	return user, nil
}

// ListUsers returns all registered users.
func (s *Store) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

// ── API Key CRUD ─────────────────────────────────────────────────────────────

// GenerateAPIKey creates a new API key for a user, returning the raw key (shown once).
// The raw key has the format: afk_<random>(32 bytes hex, prefixed).
func (s *Store) GenerateAPIKey(userID string, scopes []string, role Role, ttl time.Duration) (*APIKey, string, error) {
	if !role.Valid() {
		return nil, "", fmt.Errorf("auth: invalid role %q", role)
	}

	// Generate random key material
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("auth: generate key: %w", err)
	}
	keyPrefix := "afk_"
	keyValue := keyPrefix + hex.EncodeToString(raw)

	keyHash := hashKey(keyValue)
	now := time.Now()

	apiKey := &APIKey{
		ID:        fmt.Sprintf("apikey-%d", now.UnixNano()),
		UserID:    userID,
		KeyHash:   keyHash,
		Prefix:    keyValue[:len(keyPrefix)+8], // "afk_" + first 8 hex chars
		Scopes:    scopes,
		Role:      role,
		CreatedAt: now,
	}

	if ttl > 0 {
		apiKey.ExpiresAt = now.Add(ttl)
	}

	s.mu.Lock()
	s.keys[keyHash] = apiKey
	s.mu.Unlock()

	return apiKey, keyValue, nil
}

// ValidateAPIKey checks a raw API key string, returning the API key metadata if valid.
func (s *Store) ValidateAPIKey(rawKey string) (*APIKey, error) {
	keyHash := hashKey(rawKey)

	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, ok := s.keys[keyHash]
	if !ok {
		return nil, fmt.Errorf("auth: invalid API key")
	}

	if !apiKey.ExpiresAt.IsZero() && time.Now().After(apiKey.ExpiresAt) {
		return nil, fmt.Errorf("auth: API key expired")
	}

	return apiKey, nil
}

// RevokeAPIKey removes an API key by its ID.
func (s *Store) RevokeAPIKey(keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for hash, k := range s.keys {
		if k.ID == keyID {
			delete(s.keys, hash)
			return nil
		}
	}
	return fmt.Errorf("auth: API key %q not found", keyID)
}

// ListAPIKeys returns all API keys for a user.
func (s *Store) ListAPIKeys(userID string) []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*APIKey, 0)
	for _, k := range s.keys {
		if k.UserID == userID {
			keys = append(keys, k)
		}
	}
	return keys
}

// ── Password hashing ─────────────────────────────────────────────────────────

// hashPassword derives a hash from a password using SHA-256 with a simple
// scheme: "sha256$<salt>$<hash>". This is suitable for an internal tool
// where bcrypt/argon2 overhead is unnecessary. The password comes from
// the config capability secret which is already a high-entropy value.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	saltHex := hex.EncodeToString(salt)

	h := sha256.New()
	h.Write([]byte(saltHex))
	h.Write([]byte(password))
	hashHex := hex.EncodeToString(h.Sum(nil))

	return "sha256$" + saltHex + "$" + hashHex
}

// verifyPassword checks a password against its hash.
func verifyPassword(hash, password string) bool {
	if len(hash) < 7 || hash[:7] != "sha256$" {
		return false
	}
	parts := splitN(hash[7:], "$", 2)
	if len(parts) != 2 {
		return false
	}
	saltHex, expectedHash := parts[0], parts[1]

	h := sha256.New()
	h.Write([]byte(saltHex))
	h.Write([]byte(password))
	actualHash := hex.EncodeToString(h.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}

// hashKey hashes a raw API key for storage.
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// splitN is a simple strings.SplitN implementation for the hash format.
func splitN(s, sep string, n int) []string {
	if n <= 0 {
		return nil
	}
	var parts []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			parts = append(parts, s)
			return parts
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ── Compile-time checks ──────────────────────────────────────────────────────

var _ interface {
	AuthenticateUser(username, password string) (*User, error)
	GenerateAPIKey(userID string, scopes []string, role Role, ttl time.Duration) (*APIKey, string, error)
} = (*Store)(nil)