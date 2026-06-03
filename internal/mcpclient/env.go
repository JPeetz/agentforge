package mcpclient

import (
	"os"
	"strings"
)

// safeEnvVars is the whitelist of environment variables that are safe to pass
// to subprocess MCP servers. Blocks secrets like API keys, tokens, passwords.
var safeEnvVars = map[string]bool{
	// System paths
	"PATH":   true,
	"HOME":   true,
	"TMPDIR": true,
	"USER":   true,
	"LOGNAME": true,

	// Locale and encoding
	"LANG":    true,
	"LC_ALL":  true,
	"LC_CTYPE": true,

	// System
	"PWD":  true,
	"SHELL": true,

	// Terminal
	"TERM": true,
	"COLORTERM": true,

	// Development (safe, not secrets)
	"GOPATH":   true,
	"GOROOT":   true,
	"RUST_LOG": true,

	// Network (safe, not credentials)
	"http_proxy":  true,
	"https_proxy": true,
	"NO_PROXY":    true,
}

// blockedEnvPrefixes are prefixes of env vars that should NEVER be passed to subprocesses
// These are likely to contain secrets, API keys, tokens, credentials, etc.
var blockedEnvPrefixes = []string{
	"AWS_",
	"AZURE_",
	"GCP_",
	"GOOGLE_",
	"OPENAI_",
	"ANTHROPIC_",
	"TOKEN",
	"SECRET",
	"PASSWORD",
	"KEY",
	"CREDENTIAL",
	"AUTH",
	"API_",
	"GITHUB_",
	"GITLAB_",
	"BITBUCKET_",
	"DB_",
	"DATABASE_",
	"SLACK_",
	"DISCORD_",
	"TELEGRAM_",
	"SENDGRID_",
	"STRIPE_",
	"TWILIO_",
}

// FilterEnvironment returns a filtered environment variable list for safe subprocess execution.
// It includes only whitelisted safe variables and custom vars provided by the caller,
// while blocking any variables with secret-like names (API keys, tokens, passwords, etc.).
func FilterEnvironment(customVars map[string]string) []string {
	var env []string

	// Add whitelisted safe variables from parent process
	for _, envVar := range os.Environ() {
		key := strings.SplitN(envVar, "=", 2)[0]

		// Check if in whitelist
		if safeEnvVars[key] {
			env = append(env, envVar)
			continue
		}

		// Check if matches blocked prefix (secrets)
		if isBlocked(key) {
			continue // Skip this var, it's likely a secret
		}
	}

	// Add custom vars provided by caller
	if customVars != nil {
		for k, v := range customVars {
			// Still validate custom vars - don't allow secret-like names
			if isBlocked(k) {
				continue // Skip blocked vars even if caller provided them
			}
			env = append(env, k+"="+v)
		}
	}

	return env
}

// isBlocked checks if an environment variable name looks like it might contain secrets.
func isBlocked(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, prefix := range blockedEnvPrefixes {
		if strings.HasPrefix(upperKey, prefix) {
			return true
		}
	}
	return false
}
