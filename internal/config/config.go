// Package config — configuration management for AgentForge.
// Inspired by Hermes Agent (hermes config set), OpenClaw (env overrides),
// Tiny Claw (Zod-validated config engine), OpenHuman (managed+local config).
//
// Hierarchy: CLI flags > ENV vars > YAML file > defaults
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all AgentForge runtime configuration.
type Config struct {
	Version    string         `mapstructure:"version" json:"version"`
	Daemon     DaemonConfig   `mapstructure:"daemon" json:"daemon"`
	MCP        MCPConfig      `mapstructure:"mcp" json:"mcp"`
	GRPC       GRPCConfig     `mapstructure:"grpc" json:"grpc"`
	Memory     MemoryConfig   `mapstructure:"memory" json:"memory"`
	Providers  ProvidersConfig `mapstructure:"providers" json:"providers"`
	LLM        LLMConfig      `mapstructure:"llm" json:"llm"`
	Security   SecurityConfig `mapstructure:"security" json:"security"`
	Workers    WorkersConfig  `mapstructure:"workers" json:"workers"`
	Channels   ChannelsConfig `mapstructure:"channels" json:"channels"`
	Tools      ToolsConfig    `mapstructure:"tools" json:"tools"`
	Skills     SkillsConfig   `mapstructure:"skills" json:"skills"`
	Logging    LoggingConfig  `mapstructure:"logging" json:"logging"`
	UI         UIConfig       `mapstructure:"ui" json:"ui"`
	Pipelines  PipelinesConfig  `mapstructure:"pipelines" json:"pipelines"`
	Agents     AgentsConfig    `mapstructure:"agents" json:"agents"`
	Session    SessionConfig   `mapstructure:"session" json:"session"`

	// Convenience accessors (backward compat)
	DaemonHost string `mapstructure:"-" json:"-"`
	DaemonPort int    `mapstructure:"-" json:"-"`
	MCPPort    int    `mapstructure:"-" json:"-"` // first enabled MCP server port (convenience)
	GRPCPort   int    `mapstructure:"-" json:"-"`
}

type DaemonConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

type MCPConfig struct {
	Enabled bool        `mapstructure:"enabled" json:"enabled"`
	Servers []MCPServer `mapstructure:"servers" json:"servers"`
}

// MCPServer defines a single MCP server endpoint.
// Transport="http" starts an HTTP listener on the given port.
// Transport="stdio" spawns the Command subprocess and pipes JSON-RPC on stdin/stdout.
type MCPServer struct {
	Name      string            `mapstructure:"name" json:"name"`
	Port      int               `mapstructure:"port" json:"port"`
	Enabled   bool              `mapstructure:"enabled" json:"enabled"`
	Transport string            `mapstructure:"transport" json:"transport"`   // "http" or "stdio"
	Command   string            `mapstructure:"command,omitempty" json:"command,omitempty"`
	Args      []string          `mapstructure:"args,omitempty" json:"args,omitempty"`
	Env       map[string]string `mapstructure:"env,omitempty" json:"env,omitempty"`
	ToolFilter []string         `mapstructure:"toolFilter,omitempty" json:"toolFilter,omitempty"`
}

type GRPCConfig struct {
	Port int `mapstructure:"port" json:"port"`
}

type MemoryConfig struct {
	Root             string        `mapstructure:"root" json:"root"`
	AutoCommit       bool          `mapstructure:"autoCommit" json:"autoCommit"`
	CommitInterval   time.Duration `mapstructure:"commitInterval" json:"commitInterval"`
	IndexEnabled     bool          `mapstructure:"indexEnabled" json:"indexEnabled"`
	CompressInterval time.Duration `mapstructure:"compressInterval" json:"compressInterval"`
	MaxDailySize     string        `mapstructure:"maxDailySize" json:"maxDailySize"`
}

// ProvidersConfig holds API keys and settings per LLM provider.
// Each provider gets its own config block — like OpenClaw gateway config.
type ProvidersConfig struct {
	OpenAI      ProviderConfig `mapstructure:"openai" json:"openai"`
	Anthropic   ProviderConfig `mapstructure:"anthropic" json:"anthropic"`
	OpenRouter  ProviderConfig `mapstructure:"openrouter" json:"openrouter"`
	Google      ProviderConfig `mapstructure:"google" json:"google"`
	DeepSeek    ProviderConfig `mapstructure:"deepseek" json:"deepseek"`
	Ollama      ProviderConfig `mapstructure:"ollama" json:"ollama"`
	Groq        ProviderConfig `mapstructure:"groq" json:"groq"`
	Mistral     ProviderConfig `mapstructure:"mistral" json:"mistral"`
	Cohere      ProviderConfig `mapstructure:"cohere" json:"cohere"`
	Custom      []CustomProviderConfig `mapstructure:"custom" json:"custom"`
}

type ProviderConfig struct {
	Enabled  bool   `mapstructure:"enabled" json:"enabled"`
	APIKey   string `mapstructure:"apiKey" json:"-"`
	BaseURL  string `mapstructure:"baseUrl" json:"baseUrl,omitempty"`
	Model    string `mapstructure:"model" json:"model"`
}

type CustomProviderConfig struct {
	Name    string `mapstructure:"name" json:"name"`
	Enabled bool   `mapstructure:"enabled" json:"enabled"`
	APIKey  string `mapstructure:"apiKey" json:"-"`
	BaseURL string `mapstructure:"baseUrl" json:"baseUrl"`
	Model   string `mapstructure:"model" json:"model"`
}

type LLMConfig struct {
	Provider        string          `mapstructure:"provider" json:"provider"`
	Model           string          `mapstructure:"model" json:"model"`
	APIKey          string          `mapstructure:"apiKey" json:"-"`
	BaseURL         string          `mapstructure:"baseUrl" json:"baseUrl"`
	Timeout         time.Duration   `mapstructure:"timeout" json:"timeout"`
	Temperature     float64         `mapstructure:"temperature" json:"temperature"`
	MaxTokens       int             `mapstructure:"maxTokens" json:"maxTokens"`
	TopP            float64         `mapstructure:"topP" json:"topP"`
	FrequencyPenalty float64        `mapstructure:"frequencyPenalty" json:"frequencyPenalty"`
	PresencePenalty float64         `mapstructure:"presencePenalty" json:"presencePenalty"`
	Streaming       bool            `mapstructure:"streaming" json:"streaming"`
	Fallbacks       []LLMFallback   `mapstructure:"fallbacks" json:"fallbacks"`
	RetryCount      int             `mapstructure:"retryCount" json:"retryCount"`
	RetryDelay      time.Duration   `mapstructure:"retryDelay" json:"retryDelay"`
	RateLimit       RateLimitConfig `mapstructure:"rateLimit" json:"rateLimit"`
	Proxy           string          `mapstructure:"proxy" json:"proxy,omitempty"`
	MaxConcurrency  int             `mapstructure:"maxConcurrency" json:"maxConcurrency"`
}

type LLMFallback struct {
	Provider string `mapstructure:"provider" json:"provider"`
	Model    string `mapstructure:"model" json:"model"`
	BaseURL  string `mapstructure:"baseUrl" json:"baseUrl,omitempty"`
}

type RateLimitConfig struct {
	Enabled        bool `mapstructure:"enabled" json:"enabled"`
	RequestsPerMin int  `mapstructure:"requestsPerMin" json:"requestsPerMin"`
	TokensPerMin   int  `mapstructure:"tokensPerMin" json:"tokensPerMin"`
	BurstSize      int  `mapstructure:"burstSize" json:"burstSize"`
}

type SecurityConfig struct {
	CapabilitySecret   string        `mapstructure:"capabilitySecret" json:"-"`
	DefaultTokenBudget int64         `mapstructure:"defaultTokenBudget" json:"defaultTokenBudget"`
	DefaultTimeout     time.Duration `mapstructure:"defaultTimeout" json:"defaultTimeout"`
	EnforceOnSpawn     bool          `mapstructure:"enforceOnSpawn" json:"enforceOnSpawn"`
	EnforceOnToolCall  bool          `mapstructure:"enforceOnToolCall" json:"enforceOnToolCall"`
	AuditEnabled       bool          `mapstructure:"auditEnabled" json:"auditEnabled"`
	AllowFileSystem    bool          `mapstructure:"allowFileSystem" json:"allowFileSystem"`
	AllowNetwork       bool          `mapstructure:"allowNetwork" json:"allowNetwork"`
	AllowShell         bool          `mapstructure:"allowShell" json:"allowShell"`
	AllowBrowser       bool          `mapstructure:"allowBrowser" json:"allowBrowser"`
	SandboxMode        string        `mapstructure:"sandboxMode" json:"sandboxMode"`
	ApprovedPaths      []string      `mapstructure:"approvedPaths" json:"approvedPaths"`
	DeniedPaths        []string      `mapstructure:"deniedPaths" json:"deniedPaths"`
	AllowedDomains     []string      `mapstructure:"allowedDomains" json:"allowedDomains"`
	MaxFileSize        string        `mapstructure:"maxFileSize" json:"maxFileSize"`
	ContentFilter      bool          `mapstructure:"contentFilter" json:"contentFilter"`
}

type LoggingConfig struct {
	Level       string        `mapstructure:"level" json:"level"`
	Format      string        `mapstructure:"format" json:"format"`
	Output      string        `mapstructure:"output" json:"output"`
	MaxAge      time.Duration `mapstructure:"maxAge" json:"maxAge"`
	MaxSize     string        `mapstructure:"maxSize" json:"maxSize"`
	MaxBackups  int           `mapstructure:"maxBackups" json:"maxBackups"`
	Compress    bool          `mapstructure:"compress" json:"compress"`
	EnableTrace bool          `mapstructure:"enableTrace" json:"enableTrace"`
}

type WorkersConfig struct {
	ContentMaxAgents  int           `mapstructure:"contentMaxAgents" json:"contentMaxAgents"`
	SEOMaxAgents      int           `mapstructure:"seoMaxAgents" json:"seoMaxAgents"`
	SocialMaxAgents   int           `mapstructure:"socialMaxAgents" json:"socialMaxAgents"`
	DefaultMaxAgents  int           `mapstructure:"defaultMaxAgents" json:"defaultMaxAgents"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeatInterval" json:"heartbeatInterval"`
	MaxConcurrency    int           `mapstructure:"maxConcurrency" json:"maxConcurrency"`
	QueueSize         int           `mapstructure:"queueSize" json:"queueSize"`
	GracefulTimeout   time.Duration `mapstructure:"gracefulTimeout" json:"gracefulTimeout"`
	AutoScale         bool          `mapstructure:"autoScale" json:"autoScale"`
	ScaleUpThreshold  float64       `mapstructure:"scaleUpThreshold" json:"scaleUpThreshold"`
}

// ChannelsConfig holds messaging channel settings with full connection details.
type ChannelsConfig struct {
	Telegram TelegramConfig `mapstructure:"telegram" json:"telegram"`
	Discord  DiscordConfig  `mapstructure:"discord" json:"discord"`
	Signal   SignalConfig   `mapstructure:"signal" json:"signal"`
	WhatsApp WhatsAppConfig `mapstructure:"whatsapp" json:"whatsapp"`
	Email    EmailConfig    `mapstructure:"email" json:"email"`
	Slack    SlackConfig    `mapstructure:"slack" json:"slack"`
}

type TelegramConfig struct {
	Enabled       bool     `mapstructure:"enabled" json:"enabled"`
	BotToken      string   `mapstructure:"botToken" json:"-"`
	AllowFrom     []string `mapstructure:"allowFrom" json:"allowFrom"`
	WebhookURL    string   `mapstructure:"webhookUrl" json:"webhookUrl,omitempty"`
	PollInterval  time.Duration `mapstructure:"pollInterval" json:"pollInterval"`
	MaxFileSize   string   `mapstructure:"maxFileSize" json:"maxFileSize"`
}

type DiscordConfig struct {
	Enabled       bool     `mapstructure:"enabled" json:"enabled"`
	BotToken      string   `mapstructure:"botToken" json:"-"`
	ApplicationID string   `mapstructure:"applicationId" json:"applicationId,omitempty"`
	GuildID       string   `mapstructure:"guildId" json:"guildId,omitempty"`
	AllowChannels []string `mapstructure:"allowChannels" json:"allowChannels"`
}

type SignalConfig struct {
	Enabled       bool     `mapstructure:"enabled" json:"enabled"`
	PhoneNumber   string   `mapstructure:"phoneNumber" json:"phoneNumber,omitempty"`
	SignalCLIPath string   `mapstructure:"signalCliPath" json:"signalCliPath,omitempty"`
	AllowFrom     []string `mapstructure:"allowFrom" json:"allowFrom"`
}

type WhatsAppConfig struct {
	Enabled       bool     `mapstructure:"enabled" json:"enabled"`
	APIKey        string   `mapstructure:"apiKey" json:"-"`
	PhoneNumberID string   `mapstructure:"phoneNumberId" json:"phoneNumberId,omitempty"`
	BusinessID    string   `mapstructure:"businessId" json:"businessId,omitempty"`
	WebhookSecret string   `mapstructure:"webhookSecret" json:"-"`
	AllowFrom     []string `mapstructure:"allowFrom" json:"allowFrom"`
}

type EmailConfig struct {
	Enabled    bool   `mapstructure:"enabled" json:"enabled"`
	SMTPHost   string `mapstructure:"smtpHost" json:"smtpHost,omitempty"`
	SMTPPort   int    `mapstructure:"smtpPort" json:"smtpPort,omitempty"`
	Username   string `mapstructure:"username" json:"username,omitempty"`
	Password   string `mapstructure:"password" json:"-"`
	FromAddress string `mapstructure:"fromAddress" json:"fromAddress,omitempty"`
	UseTLS     bool   `mapstructure:"useTls" json:"useTls"`
}

type SlackConfig struct {
	Enabled       bool     `mapstructure:"enabled" json:"enabled"`
	BotToken      string   `mapstructure:"botToken" json:"-"`
	SigningSecret string   `mapstructure:"signingSecret" json:"-"`
	AppToken      string   `mapstructure:"appToken" json:"-"`
	AllowChannels []string `mapstructure:"allowChannels" json:"allowChannels"`
}

type ToolsConfig struct {
	WebSearch    bool `mapstructure:"webSearch" json:"webSearch"`
	WebFetch     bool `mapstructure:"webFetch" json:"webFetch"`
	ImageGen     bool `mapstructure:"imageGen" json:"imageGen"`
	ImageAnalyze bool `mapstructure:"imageAnalyze" json:"imageAnalyze"`
	VideoGen     bool `mapstructure:"videoGen" json:"videoGen"`
	AudioGen     bool `mapstructure:"audioGen" json:"audioGen"`
	Browser      bool `mapstructure:"browser" json:"browser"`
	CodeExec     bool `mapstructure:"codeExec" json:"codeExec"`
	GitOps       bool `mapstructure:"gitOps" json:"gitOps"`
	Cron         bool `mapstructure:"cron" json:"cron"`
	Notion       bool `mapstructure:"notion" json:"notion"`
	Calendar     bool `mapstructure:"calendar" json:"calendar"`
	Weather      bool `mapstructure:"weather" json:"weather"`
	MCPDiscovery bool `mapstructure:"mcpDiscovery" json:"mcpDiscovery"`
	DiagramGen   bool `mapstructure:"diagramGen" json:"diagramGen"`
}

type SkillsConfig struct {
	AutoInstall     bool     `mapstructure:"autoInstall" json:"autoInstall"`
	Marketplace     string   `mapstructure:"marketplace" json:"marketplace"`
	MarketplaceURL  string   `mapstructure:"marketplaceUrl" json:"marketplaceUrl"`
	MarketplaceKey  string   `mapstructure:"marketplaceKey" json:"-"`
	LocalPath       string   `mapstructure:"localPath" json:"localPath"`
	AllowAutoUpdate bool     `mapstructure:"allowAutoUpdate" json:"allowAutoUpdate"`
	MaxPerAgent     int      `mapstructure:"maxPerAgent" json:"maxPerAgent"`
	ApprovedSkills  []string `mapstructure:"approvedSkills" json:"approvedSkills"`
	BlockedKeywords []string `mapstructure:"blockedKeywords" json:"blockedKeywords"`
}

type UIConfig struct {
	Theme            string `mapstructure:"theme" json:"theme"`
	SidebarCollapsed bool   `mapstructure:"sidebarCollapsed" json:"sidebarCollapsed"`
	AutoRefreshSecs  int    `mapstructure:"autoRefreshSecs" json:"autoRefreshSecs"`
	ShowAnimations   bool   `mapstructure:"showAnimations" json:"showAnimations"`
	FontSize         string `mapstructure:"fontSize" json:"fontSize"`
	CompactMode      bool   `mapstructure:"compactMode" json:"compactMode"`
	ColorAccent      string `mapstructure:"colorAccent" json:"colorAccent"`
}

// PipelinesConfig holds pipeline orchestration definitions.
type PipelinesConfig struct {
	Definitions []PipelineDef `mapstructure:"definitions" json:"definitions"`
	AutoStart   bool          `mapstructure:"autoStart" json:"autoStart"`
	MaxConcurrent int         `mapstructure:"maxConcurrent" json:"maxConcurrent"`
	DefaultRetry  int         `mapstructure:"defaultRetry" json:"defaultRetry"`
}

type PipelineDef struct {
	Name        string        `mapstructure:"name" json:"name"`
	Enabled     bool          `mapstructure:"enabled" json:"enabled"`
	Description string        `mapstructure:"description" json:"description"`
	Trigger     PipelineTrigger `mapstructure:"trigger" json:"trigger"`
	Stages      []PipelineStage `mapstructure:"stages" json:"stages"`
}

type PipelineTrigger struct {
	Type     string `mapstructure:"type" json:"type"` // cron, event, manual
	CronExpr string `mapstructure:"cronExpr" json:"cronExpr,omitempty"`
	Event    string `mapstructure:"event" json:"event,omitempty"`
}

type PipelineStage struct {
	Name        string        `mapstructure:"name" json:"name"`
	Agent       string        `mapstructure:"agent" json:"agent"`
	Department  string        `mapstructure:"department" json:"department"`
	Tool        string        `mapstructure:"tool" json:"tool"`
	Timeout     time.Duration `mapstructure:"timeout" json:"timeout"`
	Retry       int           `mapstructure:"retry" json:"retry"`
	OnFailure   string        `mapstructure:"onFailure" json:"onFailure"` // skip, abort, escalate
	DependsOn   []string      `mapstructure:"dependsOn" json:"dependsOn"`
	Model       string        `mapstructure:"model" json:"model,omitempty"`
	Skill       string        `mapstructure:"skill" json:"skill,omitempty"`
}

// AgentsConfig holds per-agent profiles — model, tools, capability, personality.
type AgentsConfig struct {
	Profiles    []AgentProfile  `mapstructure:"profiles" json:"profiles"`
	Defaults    AgentDefaults   `mapstructure:"defaults" json:"defaults"`
}

type AgentDefaults struct {
	Model          string  `mapstructure:"model" json:"model"`
	Temperature    float64 `mapstructure:"temperature" json:"temperature"`
	MaxTokens      int     `mapstructure:"maxTokens" json:"maxTokens"`
	Timeout        time.Duration `mapstructure:"timeout" json:"timeout"`
	AllowFileSystem bool   `mapstructure:"allowFileSystem" json:"allowFileSystem"`
	AllowNetwork   bool    `mapstructure:"allowNetwork" json:"allowNetwork"`
	AllowShell     bool    `mapstructure:"allowShell" json:"allowShell"`
}

type AgentProfile struct {
	ID           string        `mapstructure:"id" json:"id"`
	Name         string        `mapstructure:"name" json:"name"`
	Department   string        `mapstructure:"department" json:"department"`
	Enabled      bool          `mapstructure:"enabled" json:"enabled"`
	Model        string        `mapstructure:"model" json:"model"`
	Provider     string        `mapstructure:"provider" json:"provider"`
	Temperature  float64       `mapstructure:"temperature" json:"temperature"`
	MaxTokens    int           `mapstructure:"maxTokens" json:"maxTokens"`
	Timeout      time.Duration `mapstructure:"timeout" json:"timeout"`
	Tools        []string      `mapstructure:"tools" json:"tools"`
	Skills       []string      `mapstructure:"skills" json:"skills"`
	Capability   AgentCapability `mapstructure:"capability" json:"capability"`
	Instructions string        `mapstructure:"instructions" json:"instructions,omitempty"`
	MemoryPath   string        `mapstructure:"memoryPath" json:"memoryPath,omitempty"`
	Heartbeat    time.Duration `mapstructure:"heartbeat" json:"heartbeat"`
}

type AgentCapability struct {
	AllowFileSystem  bool     `mapstructure:"allowFileSystem" json:"allowFileSystem"`
	AllowNetwork     bool     `mapstructure:"allowNetwork" json:"allowNetwork"`
	AllowShell       bool     `mapstructure:"allowShell" json:"allowShell"`
	AllowSpawn       bool     `mapstructure:"allowSpawn" json:"allowSpawn"`
	TokenBudget      int64    `mapstructure:"tokenBudget" json:"tokenBudget"`
	ApprovedPaths    []string `mapstructure:"approvedPaths" json:"approvedPaths"`
	DeniedPaths      []string `mapstructure:"deniedPaths" json:"deniedPaths"`
	AllowedDomains   []string `mapstructure:"allowedDomains" json:"allowedDomains"`
}

// Load reads config from YAML file, ENV vars using Viper.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("agentforge")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.agentforge")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("AGENTFORGE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.Memory.Root = os.ExpandEnv(cfg.Memory.Root)

	cfg.DaemonHost = cfg.Daemon.Host
	cfg.DaemonPort = cfg.Daemon.Port
	cfg.MCPPort = 9090 // first enabled server port; fallback default
	for _, s := range cfg.MCP.Servers {
		if s.Enabled {
			cfg.MCPPort = s.Port
			break
		}
	}
	cfg.GRPCPort = cfg.GRPC.Port

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("daemon.host", "127.0.0.1")
	v.SetDefault("daemon.port", 8080)
	v.SetDefault("mcp.enabled", false)
	v.SetDefault("grpc.port", 9091)
	v.SetDefault("memory.root", "$HOME/.agentforge/memory")
	v.SetDefault("memory.autoCommit", true)
	v.SetDefault("memory.commitInterval", "30s")
	v.SetDefault("memory.indexEnabled", true)
	v.SetDefault("memory.compressInterval", "24h")
	v.SetDefault("memory.maxDailySize", "10MB")

	// Provider defaults
	for _, p := range []string{"openai", "anthropic", "openrouter", "google", "deepseek", "ollama", "groq", "mistral", "cohere"} {
		v.SetDefault(fmt.Sprintf("providers.%s.enabled", p), false)
		v.SetDefault(fmt.Sprintf("providers.%s.baseUrl", p), "")
	}
	v.SetDefault("providers.ollama.baseUrl", "http://127.0.0.1:11434")
	v.SetDefault("providers.openai.model", "gpt-4o")
	v.SetDefault("providers.anthropic.model", "claude-sonnet-4-20250514")
	v.SetDefault("providers.openrouter.model", "openai/gpt-4o")
	v.SetDefault("providers.google.model", "gemini-2.5-pro")
	v.SetDefault("providers.deepseek.model", "deepseek-chat")
	v.SetDefault("providers.groq.model", "llama-4-maverick-17b")
	v.SetDefault("providers.mistral.model", "mistral-large-latest")
	v.SetDefault("providers.cohere.model", "command-r-plus-08-2024")

	v.SetDefault("llm.provider", "openai")
	v.SetDefault("llm.model", "gpt-4o")
	v.SetDefault("llm.timeout", "60s")
	v.SetDefault("llm.temperature", 0.7)
	v.SetDefault("llm.maxTokens", 4096)
	v.SetDefault("llm.topP", 1.0)
	v.SetDefault("llm.frequencyPenalty", 0.0)
	v.SetDefault("llm.presencePenalty", 0.0)
	v.SetDefault("llm.streaming", true)
	v.SetDefault("llm.retryCount", 3)
	v.SetDefault("llm.retryDelay", "2s")
	v.SetDefault("llm.rateLimit.enabled", true)
	v.SetDefault("llm.rateLimit.requestsPerMin", 60)
	v.SetDefault("llm.rateLimit.tokensPerMin", 200000)
	v.SetDefault("llm.rateLimit.burstSize", 5)
	v.SetDefault("llm.maxConcurrency", 10)

	v.SetDefault("security.defaultTokenBudget", 1_000_000)
	v.SetDefault("security.defaultTimeout", "3600s")
	v.SetDefault("security.enforceOnSpawn", true)
	v.SetDefault("security.enforceOnToolCall", true)
	v.SetDefault("security.auditEnabled", true)
	v.SetDefault("security.allowFileSystem", true)
	v.SetDefault("security.allowNetwork", true)
	v.SetDefault("security.allowShell", false)
	v.SetDefault("security.allowBrowser", false)
	v.SetDefault("security.sandboxMode", "non-main")
	v.SetDefault("security.maxFileSize", "10MB")
	v.SetDefault("security.contentFilter", false)

	v.SetDefault("workers.contentMaxAgents", 3)
	v.SetDefault("workers.seoMaxAgents", 1)
	v.SetDefault("workers.socialMaxAgents", 2)
	v.SetDefault("workers.defaultMaxAgents", 2)
	v.SetDefault("workers.heartbeatInterval", "30s")
	v.SetDefault("workers.maxConcurrency", 50)
	v.SetDefault("workers.queueSize", 1000)
	v.SetDefault("workers.gracefulTimeout", "30s")
	v.SetDefault("workers.autoScale", false)
	v.SetDefault("workers.scaleUpThreshold", 0.8)

	v.SetDefault("tools.webSearch", true)
	v.SetDefault("tools.webFetch", true)
	v.SetDefault("tools.imageGen", false)
	v.SetDefault("tools.imageAnalyze", false)
	v.SetDefault("tools.videoGen", false)
	v.SetDefault("tools.audioGen", false)
	v.SetDefault("tools.browser", false)
	v.SetDefault("tools.codeExec", false)
	v.SetDefault("tools.gitOps", true)
	v.SetDefault("tools.cron", true)
	v.SetDefault("tools.notion", false)
	v.SetDefault("tools.calendar", false)
	v.SetDefault("tools.weather", false)
	v.SetDefault("tools.mcpDiscovery", true)
	v.SetDefault("tools.diagramGen", false)

	v.SetDefault("skills.autoInstall", true)
	v.SetDefault("skills.marketplace", "skillsmp")
	v.SetDefault("skills.marketplaceUrl", "https://skillsmp.com/api/v1")
	v.SetDefault("skills.allowAutoUpdate", true)
	v.SetDefault("skills.maxPerAgent", 10)

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
	v.SetDefault("logging.maxAge", "168h")
	v.SetDefault("logging.maxSize", "100MB")
	v.SetDefault("logging.maxBackups", 5)
	v.SetDefault("logging.compress", true)
	v.SetDefault("logging.enableTrace", false)

	v.SetDefault("ui.theme", "volcanic-glass")
	v.SetDefault("ui.sidebarCollapsed", false)
	v.SetDefault("ui.autoRefreshSecs", 30)
	v.SetDefault("ui.showAnimations", true)

	// Session defaults — 0 context = auto-detect from LLM adapter
	v.SetDefault("session.maxContextTokens", 0)
	v.SetDefault("session.autoCompactAt", 90)
	v.SetDefault("session.keepRecentTokens", 8000)
	v.SetDefault("session.notifyUser", true)
	v.SetDefault("session.truncateAfterCompaction", true)
	v.SetDefault("session.pruneToolOutputs", true)
	v.SetDefault("session.maxToolOutputChars", 8000)
	v.SetDefault("ui.fontSize", "medium")
	v.SetDefault("ui.compactMode", false)
	v.SetDefault("ui.colorAccent", "#FF6B2C")

	v.SetDefault("pipelines.autoStart", true)
	v.SetDefault("pipelines.maxConcurrent", 3)
	v.SetDefault("pipelines.defaultRetry", 2)

	v.SetDefault("agents.defaults.model", "gpt-4o")
	v.SetDefault("agents.defaults.temperature", 0.7)
	v.SetDefault("agents.defaults.maxTokens", 4096)
	v.SetDefault("agents.defaults.timeout", "300s")
	v.SetDefault("agents.defaults.allowFileSystem", true)
	v.SetDefault("agents.defaults.allowNetwork", true)
	v.SetDefault("agents.defaults.allowShell", false)
}

func ConfigFilePath() string {
	home, _ := os.UserHomeDir()
	p := os.ExpandEnv("$HOME/.agentforge/agentforge.yaml")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return fmt.Sprintf("%s/.agentforge/agentforge.yaml", home)
}

func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "••••"
	}
	return "••••" + key[len(key)-4:]
}

func (c *Config) ToJSON() ([]byte, error) {
	return marshalConfigJSON(c), nil
}

// SetMCPDefaults ensures at least one MCP server is configured.
// Creates a "default" server on port 9090 with HTTP transport.
func SetMCPDefaults(cfg *Config) {
	if len(cfg.MCP.Servers) == 0 {
		cfg.MCP.Servers = []MCPServer{
			{
				Name:      "default",
				Port:      9090,
				Enabled:   cfg.MCP.Enabled,
				Transport: "http",
			},
		}
	}
}

func GenerateDefault() error {
	home, _ := os.UserHomeDir()
	dir := home + "/.agentforge"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	cfg, err := Load("")
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}

	cfg.Security.CapabilitySecret = ""
	cfg.LLM.APIKey = ""

	path := dir + "/agentforge.yaml"
	data := marshalConfigYAML(cfg)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Printf("Config generated: %s\n", path)
	return nil
}

// ── Provider helpers ─────────────────────────────────────────────────────────

// ProviderByStack returns the provider with the best matching model for a given stack.
func (c *Config) ProviderByStack(stack string) (ProviderConfig, bool) {
	providers := map[string]*ProviderConfig{
		"openai":     &c.Providers.OpenAI,
		"anthropic":  &c.Providers.Anthropic,
		"openrouter": &c.Providers.OpenRouter,
		"google":     &c.Providers.Google,
		"deepseek":   &c.Providers.DeepSeek,
		"ollama":     &c.Providers.Ollama,
		"groq":       &c.Providers.Groq,
		"mistral":    &c.Providers.Mistral,
		"cohere":     &c.Providers.Cohere,
	}
	if p, ok := providers[stack]; ok && p.Enabled {
		return *p, true
	}
	// Check customs
	for _, cp := range c.Providers.Custom {
		if cp.Name == stack && cp.Enabled {
			return ProviderConfig{
				Enabled: cp.Enabled,
				APIKey:  cp.APIKey,
				BaseURL: cp.BaseURL,
				Model:   cp.Model,
			}, true
		}
	}
	return ProviderConfig{}, false
}

// SessionConfig controls session lifecycle, compaction, and pruning.
type SessionConfig struct {
	MaxContextTokens   int    `mapstructure:"maxContextTokens" json:"maxContextTokens"`
	AutoCompactAt      int    `mapstructure:"autoCompactAt" json:"autoCompactAt"`
	KeepRecentTokens   int    `mapstructure:"keepRecentTokens" json:"keepRecentTokens"`
	NotifyUser         bool   `mapstructure:"notifyUser" json:"notifyUser"`
	TruncateAfter      bool   `mapstructure:"truncateAfterCompaction" json:"truncateAfterCompaction"`
	PruneToolOutputs   bool   `mapstructure:"pruneToolOutputs" json:"pruneToolOutputs"`
	MaxToolOutputChars int    `mapstructure:"maxToolOutputChars" json:"maxToolOutputChars"`
	ModelOverride      string `mapstructure:"modelOverride,omitempty" json:"modelOverride,omitempty"`
}