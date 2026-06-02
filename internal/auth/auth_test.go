package auth

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	secret := []byte("test-secret-key-for-jwt-signing")
	mgr := NewManager(ManagerConfig{
		Secret:     secret,
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		Issuer:     "agentforge-test",
	})

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.issuer != "agentforge-test" {
		t.Errorf("expected issuer 'agentforge-test', got %q", mgr.issuer)
	}
}

func TestGenerateAndValidateTokenPair(t *testing.T) {
	secret := []byte("test-secret-key")
	mgr := NewManager(ManagerConfig{
		Secret:     secret,
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		Issuer:     "agentforge",
	})

	access, refresh, err := mgr.GenerateTokenPair("user-123", RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("expected non-empty tokens")
	}
	if access == refresh {
		t.Fatal("access and refresh tokens must differ")
	}

	// Validate access token
	claims, err := mgr.ValidateToken(access)
	if err != nil {
		t.Fatalf("ValidateToken(access): %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID 'user-123', got %q", claims.UserID)
	}
	if claims.Role != RoleAdmin {
		t.Errorf("expected Role 'admin', got %q", claims.Role)
	}
	if claims.TokenType != AccessToken {
		t.Errorf("expected TokenType 'access', got %q", claims.TokenType)
	}
	if claims.Issuer != "agentforge" {
		t.Errorf("expected Issuer 'agentforge', got %q", claims.Issuer)
	}

	// Validate refresh token
	refreshClaims, err := mgr.ValidateToken(refresh)
	if err != nil {
		t.Fatalf("ValidateToken(refresh): %v", err)
	}
	if refreshClaims.TokenType != RefreshToken {
		t.Errorf("expected TokenType 'refresh', got %q", refreshClaims.TokenType)
	}
}

func TestRefreshTokenFlow(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		Secret:     []byte("refresh-test-secret"),
		AccessTTL:  1 * time.Minute,
		RefreshTTL: 1 * time.Hour,
		Issuer:     "test",
	})

	_, refresh, _ := mgr.GenerateTokenPair("user-456", RoleOperator)
	newAccess, newRefresh, err := mgr.RefreshAccessToken(refresh)
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Fatal("expected new token pair")
	}

	// Old refresh should still work (it hasn't expired)
	// New access should validate with the same user
	claims, err := mgr.ValidateToken(newAccess)
	if err != nil {
		t.Fatalf("ValidateToken(new access): %v", err)
	}
	if claims.UserID != "user-456" {
		t.Errorf("expected UserID 'user-456', got %q", claims.UserID)
	}
}

func TestRefreshWithAccessTokenFails(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		Secret:     []byte("fail-test-secret"),
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	})

	access, _, _ := mgr.GenerateTokenPair("user-789", RoleViewer)
	_, _, err := mgr.RefreshAccessToken(access)
	if err == nil {
		t.Fatal("expected error when refreshing with access token")
	}
}

func TestValidateEmptyToken(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		Secret: []byte("empty-test"),
	})

	_, err := mgr.ValidateToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestValidateInvalidToken(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		Secret: []byte("invalid-test"),
	})

	_, err := mgr.ValidateToken("not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateWithWrongSecret(t *testing.T) {
	mgr1 := NewManager(ManagerConfig{
		Secret:    []byte("secret-one"),
		AccessTTL: 15 * time.Minute,
	})
	mgr2 := NewManager(ManagerConfig{
		Secret:    []byte("secret-two"),
		AccessTTL: 15 * time.Minute,
	})

	access, _, _ := mgr1.GenerateTokenPair("user", RoleAdmin)
	_, err := mgr2.ValidateToken(access)
	if err == nil {
		t.Fatal("expected error when validating with wrong secret")
	}
}

func TestExpiredToken(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		Secret:    []byte("expired-test"),
		AccessTTL: -1 * time.Minute, // immediate expiry
	})

	access, _, _ := mgr.GenerateTokenPair("user", RoleAdmin)
	_, err := mgr.ValidateToken(access)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestRBACPermissions(t *testing.T) {
	tests := []struct {
		role     Role
		resource Resource
		action   Action
		allowed  bool
	}{
		// Admin — everything allowed
		{RoleAdmin, ResAgents, ActionRead, true},
		{RoleAdmin, ResAgents, ActionWrite, true},
		{RoleAdmin, ResAgents, ActionDelete, true},
		{RoleAdmin, ResAgents, ActionAdmin, true},
		{RoleAdmin, ResConfig, ActionWrite, true},
		{RoleAdmin, ResSecurity, ActionAdmin, true},
		{RoleAdmin, ResSession, ActionDelete, true},

		// Operator — limited
		{RoleOperator, ResAgents, ActionRead, true},
		{RoleOperator, ResAgents, ActionWrite, true},
		{RoleOperator, ResAgents, ActionDelete, false},
		{RoleOperator, ResAgents, ActionAdmin, false},
		{RoleOperator, ResLogs, ActionRead, true},
		{RoleOperator, ResLogs, ActionWrite, false},
		{RoleOperator, ResConfig, ActionRead, false},
		{RoleOperator, ResDashboard, ActionRead, true},
		{RoleOperator, ResDashboard, ActionWrite, false},

		// Viewer — read-only
		{RoleViewer, ResDashboard, ActionRead, true},
		{RoleViewer, ResAgents, ActionRead, true},
		{RoleViewer, ResAgents, ActionWrite, false},
		{RoleViewer, ResPipelines, ActionDelete, false},
		{RoleViewer, ResConfig, ActionRead, false},
		{RoleViewer, ResAuth, ActionRead, false},
		{RoleViewer, ResSecurity, ActionRead, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.role)+"-"+string(tc.resource)+"-"+string(tc.action), func(t *testing.T) {
			result := HasPermission(tc.role, tc.resource, tc.action)
			if result != tc.allowed {
				t.Errorf("HasPermission(%s, %s, %s) = %v, want %v",
					tc.role, tc.resource, tc.action, result, tc.allowed)
			}
		})
	}
}

func TestRoleValid(t *testing.T) {
	if !RoleAdmin.Valid() {
		t.Error("admin should be valid")
	}
	if !RoleOperator.Valid() {
		t.Error("operator should be valid")
	}
	if !RoleViewer.Valid() {
		t.Error("viewer should be valid")
	}
	if Role("superuser").Valid() {
		t.Error("superuser should be invalid")
	}
	if Role("").Valid() {
		t.Error("empty role should be invalid")
	}
}

func TestGenerateTokenPairInvalidRole(t *testing.T) {
	mgr := NewManager(ManagerConfig{
		Secret: []byte("role-test"),
	})
	_, _, err := mgr.GenerateTokenPair("user", Role("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestHasAtLeastRole(t *testing.T) {
	if !hasAtLeastRole(RoleAdmin, RoleAdmin) {
		t.Error("admin >= admin")
	}
	if !hasAtLeastRole(RoleAdmin, RoleOperator) {
		t.Error("admin >= operator")
	}
	if !hasAtLeastRole(RoleAdmin, RoleViewer) {
		t.Error("admin >= viewer")
	}
	if !hasAtLeastRole(RoleOperator, RoleOperator) {
		t.Error("operator >= operator")
	}
	if !hasAtLeastRole(RoleOperator, RoleViewer) {
		t.Error("operator >= viewer")
	}
	if hasAtLeastRole(RoleViewer, RoleOperator) {
		t.Error("viewer should not be >= operator")
	}
	if hasAtLeastRole(RoleViewer, RoleAdmin) {
		t.Error("viewer should not be >= admin")
	}
	if hasAtLeastRole(RoleOperator, RoleAdmin) {
		t.Error("operator should not be >= admin")
	}
}

func TestUserStore(t *testing.T) {
	store := NewStore("capability-secret-for-testing")

	// Default admin exists
	user, err := store.AuthenticateUser("admin", "capability-secret-for-testing")
	if err != nil {
		t.Fatalf("AuthenticateUser(admin): %v", err)
	}
	if user.Role != RoleAdmin {
		t.Errorf("expected admin role, got %q", user.Role)
	}

	// Wrong password
	_, err = store.AuthenticateUser("admin", "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}

	// Get by ID
	user2, err := store.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user2.Username != "admin" {
		t.Errorf("expected 'admin', got %q", user2.Username)
	}

	// Create new user
	newUser, err := store.CreateUser("operator1", "op-pass", RoleOperator)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if newUser.Username != "operator1" {
		t.Errorf("expected 'operator1', got %q", newUser.Username)
	}

	// Duplicate
	_, err = store.CreateUser("admin", "something", RoleAdmin)
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}

	// List users
	users := store.ListUsers()
	if len(users) < 2 {
		t.Errorf("expected at least 2 users, got %d", len(users))
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	store := NewStore("key-test-secret")

	apiKey, rawKey, err := store.GenerateAPIKey("admin-001", []string{"agent-1", "agent-2"}, RoleOperator, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if rawKey == "" {
		t.Fatal("expected non-empty raw key")
	}
	if !hasPrefix(rawKey, "afk_") {
		t.Errorf("expected key to start with 'afk_', got prefix %q", rawKey[:min(4, len(rawKey))])
	}

	// Validate
	validated, err := store.ValidateAPIKey(rawKey)
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if validated.ID != apiKey.ID {
		t.Errorf("expected key ID %q, got %q", apiKey.ID, validated.ID)
	}
	if validated.Role != RoleOperator {
		t.Errorf("expected role 'operator', got %q", validated.Role)
	}

	// Invalid key
	_, err = store.ValidateAPIKey("afk_badkey123")
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}

	// Revoke
	err = store.RevokeAPIKey(apiKey.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	_, err = store.ValidateAPIKey(rawKey)
	if err == nil {
		t.Fatal("expected error for revoked API key")
	}

	// List
	keys := store.ListAPIKeys("admin-001")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after revoke, got %d", len(keys))
	}
}

func TestAPIKeyExpiration(t *testing.T) {
	store := NewStore("expiry-test")

	// Generate a key with a very short TTL (1 microsecond)
	_, rawKey, err := store.GenerateAPIKey("admin-001", []string{"*"}, RoleViewer, 1*time.Microsecond)
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	// Wait a tiny bit for the key to expire
	time.Sleep(2 * time.Millisecond)

	_, err = store.ValidateAPIKey(rawKey)
	if err == nil {
		t.Fatal("expected error for expired API key")
	}
}

func TestAPIKeyNoExpiry(t *testing.T) {
	store := NewStore("no-expiry-test")

	_, rawKey, err := store.GenerateAPIKey("admin-001", []string{"*"}, RoleViewer, 0)
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	_, err = store.ValidateAPIKey(rawKey)
	if err != nil {
		t.Fatalf("ValidateAPIKey (no expiry): %v", err)
	}
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// hasPrefix checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Compile-time test for interface satisfaction (also checked in auth.go)
func TestInterfaceSatisfaction(t *testing.T) {
	cfg := DefaultManagerConfig([]byte("default-test"))
	if cfg.AccessTTL != 15*time.Minute {
		t.Errorf("expected AccessTTL 15m, got %v", cfg.AccessTTL)
	}
	if cfg.RefreshTTL != 7*24*time.Hour {
		t.Errorf("expected RefreshTTL 7d, got %v", cfg.RefreshTTL)
	}
	if cfg.Issuer != "agentforge" {
		t.Errorf("expected Issuer 'agentforge', got %q", cfg.Issuer)
	}
}

// Test that empty secret produces a fallback (warning, not panic)
func TestNilSecretFallback(t *testing.T) {
	// NewManager with empty secret should not panic
	mgr := NewManager(ManagerConfig{Secret: nil})
	if mgr == nil {
		t.Fatal("expected non-nil manager even with empty secret")
	}
	// Should still work with the fallback secret
	access, _, err := mgr.GenerateTokenPair("test", RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokenPair with fallback secret: %v", err)
	}
	if access == "" {
		t.Fatal("expected non-empty access token")
	}
}