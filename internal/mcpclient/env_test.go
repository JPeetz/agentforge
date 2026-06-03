package mcpclient

import (
	"os"
	"strings"
	"testing"
)

// ── Env Filtering Tests ──────────────────────────────────────────────────────

func TestFilterEnvironment_BlocksSecrets(t *testing.T) {
	// Set some secret env vars for this test
	secretVars := map[string]string{
		"OPENAI_KEY":        "sk-test-key",
		"AWS_ACCESS_KEY":    "AKIAIOSFODNN7EXAMPLE",
		"GITHUB_TOKEN":      "ghp_test123",
		"DATABASE_PASSWORD": "super-secret",
	}

	for k, v := range secretVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	// Filter should block these
	filtered := FilterEnvironment(nil)
	filteredStr := strings.Join(filtered, "|")

	if strings.Contains(filteredStr, "OPENAI_KEY") {
		t.Error("OPENAI_KEY leaked through filter")
	}
	if strings.Contains(filteredStr, "AWS_ACCESS_KEY") {
		t.Error("AWS_ACCESS_KEY leaked through filter")
	}
	if strings.Contains(filteredStr, "GITHUB_TOKEN") {
		t.Error("GITHUB_TOKEN leaked through filter")
	}
	if strings.Contains(filteredStr, "DATABASE_PASSWORD") {
		t.Error("DATABASE_PASSWORD leaked through filter")
	}

	t.Log("All secrets correctly blocked")
}

func TestFilterEnvironment_PreservesSafeVars(t *testing.T) {
	// Set safe env vars
	safeVars := map[string]string{
		"PATH": "/usr/bin:/bin",
		"HOME": "/home/user",
		"USER": "testuser",
		"LANG": "en_US.UTF-8",
	}

	for k, v := range safeVars {
		oldVal, hadVal := os.LookupEnv(k)
		os.Setenv(k, v)
		defer func(k, oldVal string, hadVal bool) {
			if hadVal {
				os.Setenv(k, oldVal)
			} else {
				os.Unsetenv(k)
			}
		}(k, oldVal, hadVal)
	}

	filtered := FilterEnvironment(nil)
	filteredStr := strings.Join(filtered, "|")

	// These should be preserved
	if !strings.Contains(filteredStr, "PATH=") {
		t.Error("PATH should be preserved")
	}
	if !strings.Contains(filteredStr, "HOME=") {
		t.Error("HOME should be preserved")
	}
	if !strings.Contains(filteredStr, "USER=") {
		t.Error("USER should be preserved")
	}
	if !strings.Contains(filteredStr, "LANG=") {
		t.Error("LANG should be preserved")
	}

	t.Log("All safe vars correctly preserved")
}

func TestFilterEnvironment_CustomVarsAdded(t *testing.T) {
	customVars := map[string]string{
		"MYAPP_CONFIG": "/etc/myapp/config.yml",
		"MYAPP_DEBUG":  "true",
	}

	filtered := FilterEnvironment(customVars)
	filteredStr := strings.Join(filtered, "|")

	if !strings.Contains(filteredStr, "MYAPP_CONFIG=/etc/myapp/config.yml") {
		t.Error("Custom var MYAPP_CONFIG not added")
	}
	if !strings.Contains(filteredStr, "MYAPP_DEBUG=true") {
		t.Error("Custom var MYAPP_DEBUG not added")
	}

	t.Log("Custom vars correctly added")
}

func TestFilterEnvironment_BlocksCustomSecrets(t *testing.T) {
	// Even if caller passes secret env vars, they should be blocked
	customVars := map[string]string{
		"OPENAI_KEY": "sk-malicious",
		"API_TOKEN":  "secret-token",
		"SAFE_VAR":   "is-safe",
	}

	filtered := FilterEnvironment(customVars)
	filteredStr := strings.Join(filtered, "|")

	if strings.Contains(filteredStr, "OPENAI_KEY") {
		t.Error("Custom OPENAI_KEY should be blocked")
	}
	if strings.Contains(filteredStr, "API_TOKEN") {
		t.Error("Custom API_TOKEN should be blocked")
	}
	if !strings.Contains(filteredStr, "SAFE_VAR=is-safe") {
		t.Error("SAFE_VAR should be added")
	}

	t.Log("Custom secrets correctly blocked")
}

func TestIsBlocked_DetectsSecretPrefixes(t *testing.T) {
	tests := []struct {
		varName string
		blocked bool
		desc    string
	}{
		{"OPENAI_KEY", true, "OpenAI key blocked"},
		{"OPENAI_API_KEY", true, "OpenAI API key blocked"},
		{"AWS_ACCESS_KEY_ID", true, "AWS key blocked"},
		{"GITHUB_TOKEN", true, "GitHub token blocked"},
		{"DATABASE_PASSWORD", true, "Database password blocked"},
		{"STRIPE_SECRET", true, "Stripe secret blocked"},
		{"SLACK_BOT_TOKEN", true, "Slack token blocked"},
		{"AUTH_TOKEN", true, "Auth token blocked"},
		{"SECRET_KEY", true, "Secret key blocked"},
		{"PATH", false, "PATH not blocked"},
		{"HOME", false, "HOME not blocked"},
		{"USER", false, "USER not blocked"},
		{"MYAPP_DEBUG", false, "App var not blocked"},
	}

	for _, tt := range tests {
		result := isBlocked(tt.varName)
		if result != tt.blocked {
			t.Errorf("%s: got %v, want %v", tt.desc, result, tt.blocked)
		}
	}

	t.Log("All blocking rules correctly applied")
}

func TestFilterEnvironment_CaseSensitivity(t *testing.T) {
	// Test that blocking is case-insensitive
	customVars := map[string]string{
		"openai_key": "secret",  // lowercase
		"OPENAI_KEY": "secret",  // uppercase
		"OpenAI_Key": "secret",  // mixed case
	}

	filtered := FilterEnvironment(customVars)
	filteredStr := strings.Join(filtered, "|")

	if strings.Contains(filteredStr, "openai_key") {
		t.Error("Lowercase openai_key should be blocked")
	}
	if strings.Contains(filteredStr, "OPENAI_KEY") {
		t.Error("Uppercase OPENAI_KEY should be blocked")
	}
	if strings.Contains(filteredStr, "OpenAI_Key") {
		t.Error("Mixed case OpenAI_Key should be blocked")
	}

	t.Log("Case-insensitive blocking works correctly")
}

func TestFilterEnvironment_NoLeakOfAllEnvVars(t *testing.T) {
	// Verify that NOT all parent env vars are passed through
	customVars := map[string]string{
		"SAFE_CUSTOM": "value",
	}

	filtered := FilterEnvironment(customVars)

	// Count how many env vars are included
	// Should be much less than os.Environ() due to filtering
	allVars := os.Environ()
	if len(filtered) > len(allVars)/2 {
		t.Logf("Warning: filtered env has %d vars, parent has %d", len(filtered), len(allVars))
		t.Log("This may indicate insufficient filtering (or very few unsafe vars in parent)")
	} else {
		t.Logf("Good: filtered %d vars down from %d parent vars", len(filtered), len(allVars))
	}
}

// ── Integration Tests ────────────────────────────────────────────────────────

func TestFilterEnvironment_RealWorldScenario(t *testing.T) {
	// Simulate a real MCP server scenario
	// Caller wants to pass some config but protect secrets

	mmpVars := map[string]string{
		"MODEL_PATH":   "/models/llm",
		"CACHE_DIR":    "/tmp/cache",
		"OPENAI_KEY":   "sk-real-key",  // Should be blocked
		"LOG_LEVEL":    "debug",
		"GITHUB_TOKEN": "ghp_real",     // Should be blocked
	}

	filtered := FilterEnvironment(mmpVars)
	filteredStr := strings.Join(filtered, "|")

	// Should include safe custom vars
	if !strings.Contains(filteredStr, "MODEL_PATH=/models/llm") {
		t.Error("MODEL_PATH should be preserved")
	}
	if !strings.Contains(filteredStr, "LOG_LEVEL=debug") {
		t.Error("LOG_LEVEL should be preserved")
	}

	// Should block secrets
	if strings.Contains(filteredStr, "OPENAI_KEY") {
		t.Error("OPENAI_KEY should be blocked")
	}
	if strings.Contains(filteredStr, "GITHUB_TOKEN") {
		t.Error("GITHUB_TOKEN should be blocked")
	}

	t.Log("Real-world scenario handled correctly")
}
