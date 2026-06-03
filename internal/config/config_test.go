package config

import (
	"testing"
	"time"
)

// ── Port Validation Tests ────────────────────────────────────────────────────

func TestValidate_ValidPortRange(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		MCP: MCPConfig{
			Servers: []MCPServer{
				{Name: "default", Port: 9090, Enabled: true, Transport: "http"},
			},
		},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider:       "ollama",
			Model:          "llama2",
			Timeout:        60 * time.Second,
			MaxTokens:      4096,
			MaxConcurrency: 10,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Valid config rejected: %v", err)
	}
	t.Log("Valid port range accepted")
}

func TestValidate_InvalidDaemonPort(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 99999}, // Out of range
		GRPC:   GRPCConfig{Port: 9091},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject invalid daemon port")
	}
	t.Logf("Invalid port correctly rejected: %v", err)
}

func TestValidate_InvalidGRPCPort(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 0}, // Invalid
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject invalid GRPC port")
	}
	t.Logf("Invalid GRPC port correctly rejected: %v", err)
}

func TestValidate_InvalidMCPServerPort(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		MCP: MCPConfig{
			Servers: []MCPServer{
				{Name: "bad", Port: -1, Enabled: true, Transport: "http"},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject invalid MCP server port")
	}
	t.Logf("Invalid MCP port correctly rejected: %v", err)
}

// ── Provider Validation Tests ────────────────────────────────────────────────

func TestValidate_MissingLLMProvider(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		LLM: LLMConfig{
			Provider: "", // Missing provider
			Model:    "gpt-4",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require llm.provider")
	}
	t.Logf("Missing provider correctly rejected: %v", err)
}

func TestValidate_EnabledProviderWithoutAPIKey(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			OpenAI: ProviderConfig{Enabled: true, APIKey: "", Model: "gpt-4"},
		},
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require API key for enabled providers")
	}
	t.Logf("Missing API key correctly rejected: %v", err)
}

func TestValidate_OllamaWithoutAPIKey(t *testing.T) {
	// Ollama is self-hosted and doesn't need an API key
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, APIKey: "", Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider:       "ollama",
			Model:          "llama2",
			Timeout:        60 * time.Second,
			MaxTokens:      4096,
			MaxConcurrency: 10,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Ollama without API key should be allowed: %v", err)
	}
	t.Log("Ollama self-hosted configuration accepted")
}

func TestValidate_EnabledProviderWithoutModel(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			OpenAI: ProviderConfig{Enabled: true, APIKey: "sk-test", Model: ""},
		},
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require model for enabled providers")
	}
	t.Logf("Missing model correctly rejected: %v", err)
}

func TestValidate_CustomProviderValid(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Custom: []CustomProviderConfig{
				{
					Name:     "custom-llm",
					Enabled:  true,
					APIKey:   "key-123",
					BaseURL:  "http://custom:8000",
					Model:    "custom-model",
				},
			},
		},
		LLM: LLMConfig{
			Provider:       "custom-llm",
			Model:          "custom-model",
			Timeout:        60 * time.Second,
			MaxTokens:      4096,
			MaxConcurrency: 10,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Valid custom provider rejected: %v", err)
	}
	t.Log("Valid custom provider configuration accepted")
}

// ── Channel Validation Tests ─────────────────────────────────────────────────

func TestValidate_EnabledTelegramWithoutToken(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{Enabled: true, BotToken: ""},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require Telegram bot token")
	}
	t.Logf("Missing Telegram token correctly rejected: %v", err)
}

func TestValidate_EnabledDiscordWithoutToken(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Channels: ChannelsConfig{
			Discord: DiscordConfig{Enabled: true, BotToken: ""},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require Discord bot token")
	}
	t.Logf("Missing Discord token correctly rejected: %v", err)
}

func TestValidate_EnabledEmailWithoutSMTPHost(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Channels: ChannelsConfig{
			Email: EmailConfig{Enabled: true, SMTPHost: ""},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require SMTP host for email")
	}
	t.Logf("Missing SMTP host correctly rejected: %v", err)
}

func TestValidate_EnabledEmailWithoutPort(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Channels: ChannelsConfig{
			Email: EmailConfig{
				Enabled:     true,
				SMTPHost:    "smtp.gmail.com",
				SMTPPort:    0, // Missing
				FromAddress: "bot@example.com",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require SMTP port for email")
	}
	t.Logf("Missing SMTP port correctly rejected: %v", err)
}

// ── Security Validation Tests ────────────────────────────────────────────────

func TestValidate_EnforceOnSpawnWithoutSecret(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Security: SecurityConfig{
			EnforceOnSpawn: true,
			CapabilitySecret: "", // Missing
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require capability secret when enforcement is enabled")
	}
	t.Logf("Missing secret correctly rejected: %v", err)
}

func TestValidate_InvalidSandboxMode(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Security: SecurityConfig{
			CapabilitySecret: "test-secret",
			SandboxMode:      "invalid-mode",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject invalid sandbox mode")
	}
	t.Logf("Invalid sandbox mode correctly rejected: %v", err)
}

func TestValidate_NegativeTokenBudget(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama2",
			Timeout:  60 * time.Second,
			MaxTokens: 4096,
		},
		Security: SecurityConfig{
			DefaultTokenBudget: -1,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject negative token budget")
	}
	t.Logf("Negative budget correctly rejected: %v", err)
}

// ── LLM Validation Tests ─────────────────────────────────────────────────────

func TestValidate_InvalidTemperature(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider:    "ollama",
			Model:       "llama2",
			Temperature: 5.0, // Out of range
			Timeout:     60 * time.Second,
			MaxTokens:   4096,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject invalid temperature")
	}
	t.Logf("Invalid temperature correctly rejected: %v", err)
}

func TestValidate_InvalidTopP(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider:  "ollama",
			Model:     "llama2",
			TopP:      1.5, // Out of range
			Timeout:   60 * time.Second,
			MaxTokens: 4096,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should reject invalid topP")
	}
	t.Logf("Invalid topP correctly rejected: %v", err)
}

func TestValidate_ZeroMaxTokens(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider:  "ollama",
			Model:     "llama2",
			MaxTokens: 0, // Invalid
			Timeout:   60 * time.Second,
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require positive maxTokens")
	}
	t.Logf("Invalid maxTokens correctly rejected: %v", err)
}

func TestValidate_InvalidRateLimit(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:   GRPCConfig{Port: 9091},
		Providers: ProvidersConfig{
			Ollama: ProviderConfig{Enabled: true, Model: "llama2"},
		},
		LLM: LLMConfig{
			Provider:  "ollama",
			Model:     "llama2",
			Timeout:   60 * time.Second,
			MaxTokens: 4096,
			RateLimit: RateLimitConfig{
				Enabled:        true,
				RequestsPerMin: 0, // Invalid
			},
		},
		Security: SecurityConfig{
			DefaultTimeout: 3600 * time.Second,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Should require positive requests per minute")
	}
	t.Logf("Invalid rate limit correctly rejected: %v", err)
}

// ── Integration Tests ────────────────────────────────────────────────────────

func TestValidate_FullValidConfiguration(t *testing.T) {
	cfg := &Config{
		Version: "1.0.0",
		Daemon:  DaemonConfig{Host: "127.0.0.1", Port: 8080},
		GRPC:    GRPCConfig{Port: 9091},
		MCP: MCPConfig{
			Enabled: true,
			Servers: []MCPServer{
				{
					Name:      "default",
					Port:      9090,
					Enabled:   true,
					Transport: "http",
				},
			},
		},
		Providers: ProvidersConfig{
			OpenAI: ProviderConfig{
				Enabled: true,
				APIKey:  "sk-test-key",
				Model:   "gpt-4o",
			},
		},
		LLM: LLMConfig{
			Provider:      "openai",
			Model:         "gpt-4o",
			APIKey:        "sk-test-key",
			Timeout:       60 * time.Second,
			Temperature:   0.7,
			MaxTokens:     4096,
			TopP:          1.0,
			RetryCount:    3,
			MaxConcurrency: 10,
			RateLimit: RateLimitConfig{
				Enabled:        true,
				RequestsPerMin: 60,
				TokensPerMin:   200000,
			},
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{Enabled: false},
		},
		Security: SecurityConfig{
			CapabilitySecret:   "test-secret",
			DefaultTokenBudget: 1000000,
			DefaultTimeout:     3600 * time.Second,
			EnforceOnSpawn:     true,
			SandboxMode:        "non-main",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Valid configuration rejected: %v", err)
	}
	t.Log("Full valid configuration accepted")
}
