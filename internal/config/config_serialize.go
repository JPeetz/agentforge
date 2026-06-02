// config_serialize.go — JSON and YAML serialization with secret masking.
package config

import (
	"encoding/json"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ── JSON Serialization (dashboard API, config list) ──────────────────────────

func marshalConfigJSON(c *Config) []byte {
	out := map[string]any{
		"version": c.Version,
		"daemon":  map[string]any{"host": c.Daemon.Host, "port": c.Daemon.Port},
		"mcp":     mcpJSON(c),
		"grpc":    map[string]any{"port": c.GRPC.Port},
		"memory":  memoryJSON(c),
		"providers": providersJSON(c),
		"llm":      llmJSON(c),
		"security": securityJSON(c),
		"workers":  workersJSON(c),
		"channels": channelsJSON(c),
		"tools":    toolsJSON(c),
		"skills":   skillsJSON(c),
		"logging":  loggingJSON(c),
		"ui":       uiJSON(c),
		"pipelines": pipelinesJSON(c),
		"agents":    agentsJSON(c),
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	return data
}

func memoryJSON(c *Config) map[string]any {
	return map[string]any{
		"root": c.Memory.Root, "autoCommit": c.Memory.AutoCommit,
		"commitInterval": dur(c.Memory.CommitInterval),
		"indexEnabled": c.Memory.IndexEnabled,
		"compressInterval": dur(c.Memory.CompressInterval),
		"maxDailySize": c.Memory.MaxDailySize,
	}
}

func mcpJSON(c *Config) map[string]any {
	servers := []map[string]any{}
	for _, s := range c.MCP.Servers {
		srv := map[string]any{
			"name":      s.Name,
			"port":      s.Port,
			"enabled":   s.Enabled,
			"transport": s.Transport,
		}
		if len(s.ToolFilter) > 0 {
			srv["toolFilter"] = s.ToolFilter
		}
		servers = append(servers, srv)
	}
	return map[string]any{
		"enabled": c.MCP.Enabled,
		"servers": servers,
	}
}

func providersJSON(c *Config) map[string]any {
	p := map[string]any{}
	provMap := map[string]ProviderConfig{
		"openai": c.Providers.OpenAI, "anthropic": c.Providers.Anthropic,
		"openrouter": c.Providers.OpenRouter, "google": c.Providers.Google,
		"deepseek": c.Providers.DeepSeek, "ollama": c.Providers.Ollama,
		"groq": c.Providers.Groq, "mistral": c.Providers.Mistral,
		"cohere": c.Providers.Cohere,
	}
	for name, pc := range provMap {
		p[name] = map[string]any{
			"enabled": pc.Enabled, "apiKey": MaskAPIKey(pc.APIKey),
			"baseUrl": pc.BaseURL, "model": pc.Model,
		}
	}
	if len(c.Providers.Custom) > 0 {
		customs := []map[string]any{}
		for _, cp := range c.Providers.Custom {
			customs = append(customs, map[string]any{
				"name": cp.Name, "enabled": cp.Enabled,
				"apiKey": MaskAPIKey(cp.APIKey), "baseUrl": cp.BaseURL, "model": cp.Model,
			})
		}
		p["custom"] = customs
	}
	return p
}

func llmJSON(c *Config) map[string]any {
	fb := []map[string]any{}
	for _, f := range c.LLM.Fallbacks {
		fb = append(fb, map[string]any{"provider": f.Provider, "model": f.Model, "baseUrl": f.BaseURL})
	}
	return map[string]any{
		"provider": c.LLM.Provider, "model": c.LLM.Model,
		"apiKey": MaskAPIKey(c.LLM.APIKey), "baseUrl": c.LLM.BaseURL,
		"timeout": dur(c.LLM.Timeout), "temperature": c.LLM.Temperature,
		"maxTokens": c.LLM.MaxTokens, "topP": c.LLM.TopP,
		"frequencyPenalty": c.LLM.FrequencyPenalty, "presencePenalty": c.LLM.PresencePenalty,
		"streaming": c.LLM.Streaming, "fallbacks": fb,
		"retryCount": c.LLM.RetryCount, "retryDelay": dur(c.LLM.RetryDelay),
		"rateLimit": map[string]any{
			"enabled": c.LLM.RateLimit.Enabled, "requestsPerMin": c.LLM.RateLimit.RequestsPerMin,
			"tokensPerMin": c.LLM.RateLimit.TokensPerMin, "burstSize": c.LLM.RateLimit.BurstSize,
		},
		"proxy": c.LLM.Proxy, "maxConcurrency": c.LLM.MaxConcurrency,
	}
}

func securityJSON(c *Config) map[string]any {
	return map[string]any{
		"defaultTokenBudget": c.Security.DefaultTokenBudget,
		"defaultTimeout": dur(c.Security.DefaultTimeout),
		"enforceOnSpawn": c.Security.EnforceOnSpawn,
		"enforceOnToolCall": c.Security.EnforceOnToolCall,
		"auditEnabled": c.Security.AuditEnabled,
		"allowFileSystem": c.Security.AllowFileSystem, "allowNetwork": c.Security.AllowNetwork,
		"allowShell": c.Security.AllowShell, "allowBrowser": c.Security.AllowBrowser,
		"sandboxMode": c.Security.SandboxMode,
		"approvedPaths": c.Security.ApprovedPaths, "deniedPaths": c.Security.DeniedPaths,
		"allowedDomains": c.Security.AllowedDomains, "maxFileSize": c.Security.MaxFileSize,
		"contentFilter": c.Security.ContentFilter,
	}
}

func workersJSON(c *Config) map[string]any {
	return map[string]any{
		"contentMaxAgents": c.Workers.ContentMaxAgents,
		"seoMaxAgents": c.Workers.SEOMaxAgents,
		"socialMaxAgents": c.Workers.SocialMaxAgents,
		"defaultMaxAgents": c.Workers.DefaultMaxAgents,
		"heartbeatInterval": dur(c.Workers.HeartbeatInterval),
		"maxConcurrency": c.Workers.MaxConcurrency,
		"queueSize": c.Workers.QueueSize,
		"gracefulTimeout": dur(c.Workers.GracefulTimeout),
		"autoScale": c.Workers.AutoScale,
		"scaleUpThreshold": c.Workers.ScaleUpThreshold,
	}
}

func channelsJSON(c *Config) map[string]any {
	return map[string]any{
		"telegram": map[string]any{
			"enabled": c.Channels.Telegram.Enabled,
			"botToken": MaskAPIKey(c.Channels.Telegram.BotToken),
			"allowFrom": c.Channels.Telegram.AllowFrom,
			"webhookUrl": c.Channels.Telegram.WebhookURL,
			"pollInterval": dur(c.Channels.Telegram.PollInterval),
			"maxFileSize": c.Channels.Telegram.MaxFileSize,
		},
		"discord": map[string]any{
			"enabled": c.Channels.Discord.Enabled,
			"botToken": MaskAPIKey(c.Channels.Discord.BotToken),
			"applicationId": c.Channels.Discord.ApplicationID,
			"guildId": c.Channels.Discord.GuildID,
			"allowChannels": c.Channels.Discord.AllowChannels,
		},
		"signal": map[string]any{
			"enabled": c.Channels.Signal.Enabled,
			"phoneNumber": c.Channels.Signal.PhoneNumber,
			"signalCliPath": c.Channels.Signal.SignalCLIPath,
			"allowFrom": c.Channels.Signal.AllowFrom,
		},
		"whatsapp": map[string]any{
			"enabled": c.Channels.WhatsApp.Enabled,
			"apiKey": MaskAPIKey(c.Channels.WhatsApp.APIKey),
			"phoneNumberId": c.Channels.WhatsApp.PhoneNumberID,
			"businessId": c.Channels.WhatsApp.BusinessID,
			"allowFrom": c.Channels.WhatsApp.AllowFrom,
		},
		"email": map[string]any{
			"enabled": c.Channels.Email.Enabled,
			"smtpHost": c.Channels.Email.SMTPHost,
			"smtpPort": c.Channels.Email.SMTPPort,
			"username": c.Channels.Email.Username,
			"fromAddress": c.Channels.Email.FromAddress,
			"useTls": c.Channels.Email.UseTLS,
		},
		"slack": map[string]any{
			"enabled": c.Channels.Slack.Enabled,
			"botToken": MaskAPIKey(c.Channels.Slack.BotToken),
			"allowChannels": c.Channels.Slack.AllowChannels,
		},
	}
}

func toolsJSON(c *Config) map[string]any {
	return map[string]any{
		"webSearch": c.Tools.WebSearch, "webFetch": c.Tools.WebFetch,
		"imageGen": c.Tools.ImageGen, "imageAnalyze": c.Tools.ImageAnalyze,
		"videoGen": c.Tools.VideoGen, "audioGen": c.Tools.AudioGen,
		"browser": c.Tools.Browser, "codeExec": c.Tools.CodeExec,
		"gitOps": c.Tools.GitOps, "cron": c.Tools.Cron,
		"notion": c.Tools.Notion, "calendar": c.Tools.Calendar,
		"weather": c.Tools.Weather, "mcpDiscovery": c.Tools.MCPDiscovery,
		"diagramGen": c.Tools.DiagramGen,
	}
}

func skillsJSON(c *Config) map[string]any {
	return map[string]any{
		"autoInstall": c.Skills.AutoInstall, "marketplace": c.Skills.Marketplace,
		"marketplaceUrl": c.Skills.MarketplaceURL, "marketplaceKey": MaskAPIKey(c.Skills.MarketplaceKey),
		"localPath": c.Skills.LocalPath, "allowAutoUpdate": c.Skills.AllowAutoUpdate,
		"maxPerAgent": c.Skills.MaxPerAgent,
		"approvedSkills": c.Skills.ApprovedSkills, "blockedKeywords": c.Skills.BlockedKeywords,
	}
}

func loggingJSON(c *Config) map[string]any {
	return map[string]any{
		"level": c.Logging.Level, "format": c.Logging.Format, "output": c.Logging.Output,
		"maxAge": dur(c.Logging.MaxAge), "maxSize": c.Logging.MaxSize,
		"maxBackups": c.Logging.MaxBackups, "compress": c.Logging.Compress,
		"enableTrace": c.Logging.EnableTrace,
	}
}

func uiJSON(c *Config) map[string]any {
	return map[string]any{
		"theme": c.UI.Theme, "sidebarCollapsed": c.UI.SidebarCollapsed,
		"autoRefreshSecs": c.UI.AutoRefreshSecs, "showAnimations": c.UI.ShowAnimations,
		"fontSize": c.UI.FontSize, "compactMode": c.UI.CompactMode,
		"colorAccent": c.UI.ColorAccent,
	}
}

func pipelinesJSON(c *Config) map[string]any {
	defs := []map[string]any{}
	for _, d := range c.Pipelines.Definitions {
		stages := []map[string]any{}
		for _, st := range d.Stages {
			stage := map[string]any{
				"name": st.Name, "agent": st.Agent, "department": st.Department,
				"tool": st.Tool, "timeout": dur(st.Timeout), "retry": st.Retry,
				"onFailure": st.OnFailure, "dependsOn": st.DependsOn,
				"model": st.Model, "skill": st.Skill,
			}
			stages = append(stages, stage)
		}
		defs = append(defs, map[string]any{
			"name": d.Name, "enabled": d.Enabled, "description": d.Description,
			"trigger": map[string]any{"type": d.Trigger.Type, "cronExpr": d.Trigger.CronExpr, "event": d.Trigger.Event},
			"stages": stages,
		})
	}
	return map[string]any{
		"definitions": defs, "autoStart": c.Pipelines.AutoStart,
		"maxConcurrent": c.Pipelines.MaxConcurrent, "defaultRetry": c.Pipelines.DefaultRetry,
	}
}

func agentsJSON(c *Config) map[string]any {
	profiles := []map[string]any{}
	for _, ap := range c.Agents.Profiles {
		profiles = append(profiles, map[string]any{
			"id": ap.ID, "name": ap.Name, "department": ap.Department,
			"enabled": ap.Enabled, "model": ap.Model, "provider": ap.Provider,
			"temperature": ap.Temperature, "maxTokens": ap.MaxTokens,
			"timeout": dur(ap.Timeout), "tools": ap.Tools, "skills": ap.Skills,
			"capability": map[string]any{
				"allowFileSystem": ap.Capability.AllowFileSystem,
				"allowNetwork": ap.Capability.AllowNetwork,
				"allowShell": ap.Capability.AllowShell,
				"allowSpawn": ap.Capability.AllowSpawn,
				"tokenBudget": ap.Capability.TokenBudget,
				"approvedPaths": ap.Capability.ApprovedPaths,
				"deniedPaths": ap.Capability.DeniedPaths,
				"allowedDomains": ap.Capability.AllowedDomains,
			},
			"instructions": ap.Instructions, "memoryPath": ap.MemoryPath,
			"heartbeat": dur(ap.Heartbeat),
		})
	}
	return map[string]any{
		"profiles": profiles,
		"defaults": map[string]any{
			"model": c.Agents.Defaults.Model, "temperature": c.Agents.Defaults.Temperature,
			"maxTokens": c.Agents.Defaults.MaxTokens, "timeout": dur(c.Agents.Defaults.Timeout),
			"allowFileSystem": c.Agents.Defaults.AllowFileSystem,
			"allowNetwork": c.Agents.Defaults.AllowNetwork,
			"allowShell": c.Agents.Defaults.AllowShell,
		},
	}
}

func dur(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.String()
}

// ── YAML Serialization (config generate) ─────────────────────────────────────

type yamlDoc struct {
	Version   string      `yaml:"version"`
	Daemon    yamlDaemon  `yaml:"daemon"`
	MCP       yamlMCP     `yaml:"mcp"`
	GRPC      yamlGRPC    `yaml:"grpc"`
	Memory    yamlMemory  `yaml:"memory"`
	Providers yamlProviders `yaml:"providers"`
	LLM       yamlLLM     `yaml:"llm"`
	Security  yamlSecurity `yaml:"security"`
	Workers   yamlWorkers `yaml:"workers"`
	Channels  yamlChannels `yaml:"channels"`
	Tools     yamlTools   `yaml:"tools"`
	Skills    yamlSkills  `yaml:"skills"`
	Logging   yamlLogging `yaml:"logging"`
	UI        yamlUI      `yaml:"ui"`
	Pipelines yamlPipelines `yaml:"pipelines"`
	Agents    yamlAgents  `yaml:"agents"`
}

type yamlDaemon struct{ Host string `yaml:"host"`; Port int `yaml:"port"` }
type yamlMCP struct {
	Enabled bool        `yaml:"enabled"`
	Servers []yamlMCPServer `yaml:"servers"`
}

type yamlMCPServer struct {
	Name       string            `yaml:"name"`
	Port       int               `yaml:"port"`
	Enabled    bool              `yaml:"enabled"`
	Transport  string            `yaml:"transport"`
	Command    string            `yaml:"command,omitempty"`
	Args       []string          `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	ToolFilter []string          `yaml:"toolFilter,omitempty"`
}
type yamlGRPC struct{ Port int `yaml:"port"` }
type yamlMemory struct {
	Root string `yaml:"root"`; AutoCommit bool `yaml:"autoCommit"`
	CommitInterval string `yaml:"commitInterval"`; IndexEnabled bool `yaml:"indexEnabled"`
	CompressInterval string `yaml:"compressInterval"`; MaxDailySize string `yaml:"maxDailySize"`
}
type yamlProviders struct {
	OpenAI ProviderDef `yaml:"openai"`; Anthropic ProviderDef `yaml:"anthropic"`
	OpenRouter ProviderDef `yaml:"openrouter"`; Google ProviderDef `yaml:"google"`
	DeepSeek ProviderDef `yaml:"deepseek"`; Ollama ProviderDef `yaml:"ollama"`
	Groq ProviderDef `yaml:"groq"`; Mistral ProviderDef `yaml:"mistral"`
	Cohere ProviderDef `yaml:"cohere"`; Custom []CustomProviderDef `yaml:"custom"`
}
type ProviderDef struct {
	Enabled bool `yaml:"enabled"`; APIKey string `yaml:"apiKey"`
	BaseURL string `yaml:"baseUrl,omitempty"`; Model string `yaml:"model"`
}
type CustomProviderDef struct {
	Name string `yaml:"name"`; Enabled bool `yaml:"enabled"`
	APIKey string `yaml:"apiKey"`; BaseURL string `yaml:"baseUrl"`; Model string `yaml:"model"`
}
type yamlLLM struct {
	Provider string `yaml:"provider"`; Model string `yaml:"model"`
	APIKey string `yaml:"apiKey"`; BaseURL string `yaml:"baseUrl"`
	Timeout string `yaml:"timeout"`; Temperature float64 `yaml:"temperature"`
	MaxTokens int `yaml:"maxTokens"`; TopP float64 `yaml:"topP"`
	FrequencyPenalty float64 `yaml:"frequencyPenalty"`; PresencePenalty float64 `yaml:"presencePenalty"`
	Streaming bool `yaml:"streaming"`; RetryCount int `yaml:"retryCount"`
	RetryDelay string `yaml:"retryDelay"`
	RateLimit yamlRateLimit `yaml:"rateLimit"`
	Proxy string `yaml:"proxy,omitempty"`; MaxConcurrency int `yaml:"maxConcurrency"`
}
type yamlRateLimit struct {
	Enabled bool `yaml:"enabled"`; RequestsPerMin int `yaml:"requestsPerMin"`
	TokensPerMin int `yaml:"tokensPerMin"`; BurstSize int `yaml:"burstSize"`
}
type yamlSecurity struct {
	CapabilitySecret string `yaml:"capabilitySecret"`; DefaultTokenBudget int64 `yaml:"defaultTokenBudget"`
	DefaultTimeout string `yaml:"defaultTimeout"`; EnforceOnSpawn bool `yaml:"enforceOnSpawn"`
	EnforceOnToolCall bool `yaml:"enforceOnToolCall"`; AuditEnabled bool `yaml:"auditEnabled"`
	AllowFileSystem bool `yaml:"allowFileSystem"`; AllowNetwork bool `yaml:"allowNetwork"`
	AllowShell bool `yaml:"allowShell"`; AllowBrowser bool `yaml:"allowBrowser"`
	SandboxMode string `yaml:"sandboxMode"`; ApprovedPaths []string `yaml:"approvedPaths"`
	DeniedPaths []string `yaml:"deniedPaths"`; AllowedDomains []string `yaml:"allowedDomains"`
	MaxFileSize string `yaml:"maxFileSize"`; ContentFilter bool `yaml:"contentFilter"`
}
type yamlWorkers struct {
	ContentMaxAgents int `yaml:"contentMaxAgents"`; SEOMaxAgents int `yaml:"seoMaxAgents"`
	SocialMaxAgents int `yaml:"socialMaxAgents"`; DefaultMaxAgents int `yaml:"defaultMaxAgents"`
	HeartbeatInterval string `yaml:"heartbeatInterval"`; MaxConcurrency int `yaml:"maxConcurrency"`
	QueueSize int `yaml:"queueSize"`; GracefulTimeout string `yaml:"gracefulTimeout"`
	AutoScale bool `yaml:"autoScale"`; ScaleUpThreshold float64 `yaml:"scaleUpThreshold"`
}
type yamlChannels struct {
	Telegram yamlTelegram `yaml:"telegram"`; Discord yamlDiscord `yaml:"discord"`
	Signal yamlSignal `yaml:"signal"`; WhatsApp yamlWhatsApp `yaml:"whatsapp"`
	Email yamlEmail `yaml:"email"`; Slack yamlSlack `yaml:"slack"`
}
type yamlTelegram struct {
	Enabled bool `yaml:"enabled"`; BotToken string `yaml:"botToken"`
	AllowFrom []string `yaml:"allowFrom"`; WebhookURL string `yaml:"webhookUrl"`
	PollInterval string `yaml:"pollInterval"`; MaxFileSize string `yaml:"maxFileSize"`
}
type yamlDiscord struct {
	Enabled bool `yaml:"enabled"`; BotToken string `yaml:"botToken"`
	ApplicationID string `yaml:"applicationId"`; GuildID string `yaml:"guildId"`
	AllowChannels []string `yaml:"allowChannels"`
}
type yamlSignal struct {
	Enabled bool `yaml:"enabled"`; PhoneNumber string `yaml:"phoneNumber"`
	SignalCLIPath string `yaml:"signalCliPath"`; AllowFrom []string `yaml:"allowFrom"`
}
type yamlWhatsApp struct {
	Enabled bool `yaml:"enabled"`; APIKey string `yaml:"apiKey"`
	PhoneNumberID string `yaml:"phoneNumberId"`; BusinessID string `yaml:"businessId"`
	WebhookSecret string `yaml:"webhookSecret"`; AllowFrom []string `yaml:"allowFrom"`
}
type yamlEmail struct {
	Enabled bool `yaml:"enabled"`; SMTPHost string `yaml:"smtpHost"`
	SMTPPort int `yaml:"smtpPort"`; Username string `yaml:"username"`
	Password string `yaml:"password"`; FromAddress string `yaml:"fromAddress"`
	UseTLS bool `yaml:"useTls"`
}
type yamlSlack struct {
	Enabled bool `yaml:"enabled"`; BotToken string `yaml:"botToken"`
	SigningSecret string `yaml:"signingSecret"`; AppToken string `yaml:"appToken"`
	AllowChannels []string `yaml:"allowChannels"`
}
type yamlTools struct {
	WebSearch bool `yaml:"webSearch"`; WebFetch bool `yaml:"webFetch"`
	ImageGen bool `yaml:"imageGen"`; ImageAnalyze bool `yaml:"imageAnalyze"`
	VideoGen bool `yaml:"videoGen"`; AudioGen bool `yaml:"audioGen"`
	Browser bool `yaml:"browser"`; CodeExec bool `yaml:"codeExec"`
	GitOps bool `yaml:"gitOps"`; Cron bool `yaml:"cron"`
	Notion bool `yaml:"notion"`; Calendar bool `yaml:"calendar"`
	Weather bool `yaml:"weather"`; MCPDiscovery bool `yaml:"mcpDiscovery"`
	DiagramGen bool `yaml:"diagramGen"`
}
type yamlSkills struct {
	AutoInstall bool `yaml:"autoInstall"`; Marketplace string `yaml:"marketplace"`
	MarketplaceURL string `yaml:"marketplaceUrl"`; MarketplaceKey string `yaml:"marketplaceKey"`
	LocalPath string `yaml:"localPath"`; AllowAutoUpdate bool `yaml:"allowAutoUpdate"`
	MaxPerAgent int `yaml:"maxPerAgent"`; ApprovedSkills []string `yaml:"approvedSkills"`
	BlockedKeywords []string `yaml:"blockedKeywords"`
}
type yamlLogging struct {
	Level string `yaml:"level"`; Format string `yaml:"format"`; Output string `yaml:"output"`
	MaxAge string `yaml:"maxAge"`; MaxSize string `yaml:"maxSize"`
	MaxBackups int `yaml:"maxBackups"`; Compress bool `yaml:"compress"`
	EnableTrace bool `yaml:"enableTrace"`
}
type yamlUI struct {
	Theme string `yaml:"theme"`; SidebarCollapsed bool `yaml:"sidebarCollapsed"`
	AutoRefreshSecs int `yaml:"autoRefreshSecs"`; ShowAnimations bool `yaml:"showAnimations"`
	FontSize string `yaml:"fontSize"`; CompactMode bool `yaml:"compactMode"`
	ColorAccent string `yaml:"colorAccent"`
}
type yamlPipelines struct {
	AutoStart bool `yaml:"autoStart"`; MaxConcurrent int `yaml:"maxConcurrent"`
	DefaultRetry int `yaml:"defaultRetry"`
}
type yamlAgents struct{}

func marshalConfigYAML(c *Config) []byte {
	doc := fillYAMLDoc(c)
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return []byte("# Error generating YAML\n")
	}
	return []byte(buf.String())
}

func fillYAMLDoc(c *Config) yamlDoc {
	return yamlDoc{
		Version: c.Version,
		Daemon:  yamlDaemon{Host: c.Daemon.Host, Port: c.Daemon.Port},
		MCP:     yamlMCPServers(c),
		GRPC:    yamlGRPC{Port: c.GRPC.Port},
		Memory: yamlMemory{
			Root: c.Memory.Root, AutoCommit: c.Memory.AutoCommit,
			CommitInterval: dur(c.Memory.CommitInterval), IndexEnabled: c.Memory.IndexEnabled,
			CompressInterval: dur(c.Memory.CompressInterval), MaxDailySize: c.Memory.MaxDailySize,
		},
		Providers: yamlProviders{
			OpenAI: pd(c.Providers.OpenAI), Anthropic: pd(c.Providers.Anthropic),
			OpenRouter: pd(c.Providers.OpenRouter), Google: pd(c.Providers.Google),
			DeepSeek: pd(c.Providers.DeepSeek), Ollama: pd(c.Providers.Ollama),
			Groq: pd(c.Providers.Groq), Mistral: pd(c.Providers.Mistral),
			Cohere: pd(c.Providers.Cohere),
		},
		LLM: yamlLLM{
			Provider: c.LLM.Provider, Model: c.LLM.Model, APIKey: "",
			BaseURL: c.LLM.BaseURL, Timeout: dur(c.LLM.Timeout),
			Temperature: c.LLM.Temperature, MaxTokens: c.LLM.MaxTokens,
			TopP: c.LLM.TopP, FrequencyPenalty: c.LLM.FrequencyPenalty,
			PresencePenalty: c.LLM.PresencePenalty, Streaming: c.LLM.Streaming,
			RetryCount: c.LLM.RetryCount, RetryDelay: dur(c.LLM.RetryDelay),
			RateLimit: yamlRateLimit{
				Enabled: c.LLM.RateLimit.Enabled, RequestsPerMin: c.LLM.RateLimit.RequestsPerMin,
				TokensPerMin: c.LLM.RateLimit.TokensPerMin, BurstSize: c.LLM.RateLimit.BurstSize,
			},
			Proxy: c.LLM.Proxy, MaxConcurrency: c.LLM.MaxConcurrency,
		},
		Security: yamlSecurity{
			CapabilitySecret: "auto-generated", DefaultTokenBudget: c.Security.DefaultTokenBudget,
			DefaultTimeout: dur(c.Security.DefaultTimeout), EnforceOnSpawn: c.Security.EnforceOnSpawn,
			EnforceOnToolCall: c.Security.EnforceOnToolCall, AuditEnabled: c.Security.AuditEnabled,
			AllowFileSystem: c.Security.AllowFileSystem, AllowNetwork: c.Security.AllowNetwork,
			AllowShell: c.Security.AllowShell, AllowBrowser: c.Security.AllowBrowser,
			SandboxMode: c.Security.SandboxMode, ApprovedPaths: c.Security.ApprovedPaths,
			DeniedPaths: c.Security.DeniedPaths, AllowedDomains: c.Security.AllowedDomains,
			MaxFileSize: c.Security.MaxFileSize, ContentFilter: c.Security.ContentFilter,
		},
		Workers: yamlWorkers{
			ContentMaxAgents: c.Workers.ContentMaxAgents, SEOMaxAgents: c.Workers.SEOMaxAgents,
			SocialMaxAgents: c.Workers.SocialMaxAgents, DefaultMaxAgents: c.Workers.DefaultMaxAgents,
			HeartbeatInterval: dur(c.Workers.HeartbeatInterval), MaxConcurrency: c.Workers.MaxConcurrency,
			QueueSize: c.Workers.QueueSize, GracefulTimeout: dur(c.Workers.GracefulTimeout),
			AutoScale: c.Workers.AutoScale, ScaleUpThreshold: c.Workers.ScaleUpThreshold,
		},
		Channels: yamlChannels{
			Telegram: yamlTelegram{
				Enabled: c.Channels.Telegram.Enabled, BotToken: "", AllowFrom: c.Channels.Telegram.AllowFrom,
				WebhookURL: c.Channels.Telegram.WebhookURL, PollInterval: dur(c.Channels.Telegram.PollInterval),
				MaxFileSize: c.Channels.Telegram.MaxFileSize,
			},
			Discord: yamlDiscord{
				Enabled: c.Channels.Discord.Enabled, BotToken: "", ApplicationID: c.Channels.Discord.ApplicationID,
				GuildID: c.Channels.Discord.GuildID, AllowChannels: c.Channels.Discord.AllowChannels,
			},
			Signal: yamlSignal{
				Enabled: c.Channels.Signal.Enabled, PhoneNumber: "", SignalCLIPath: c.Channels.Signal.SignalCLIPath,
				AllowFrom: c.Channels.Signal.AllowFrom,
			},
			WhatsApp: yamlWhatsApp{
				Enabled: c.Channels.WhatsApp.Enabled, APIKey: "", PhoneNumberID: c.Channels.WhatsApp.PhoneNumberID,
				BusinessID: c.Channels.WhatsApp.BusinessID, WebhookSecret: "", AllowFrom: c.Channels.WhatsApp.AllowFrom,
			},
			Email: yamlEmail{
				Enabled: c.Channels.Email.Enabled, SMTPHost: c.Channels.Email.SMTPHost,
				SMTPPort: c.Channels.Email.SMTPPort, Username: c.Channels.Email.Username,
				Password: "", FromAddress: c.Channels.Email.FromAddress, UseTLS: c.Channels.Email.UseTLS,
			},
			Slack: yamlSlack{
				Enabled: c.Channels.Slack.Enabled, BotToken: "", SigningSecret: "",
				AppToken: "", AllowChannels: c.Channels.Slack.AllowChannels,
			},
		},
		Tools: yamlTools{
			WebSearch: c.Tools.WebSearch, WebFetch: c.Tools.WebFetch,
			ImageGen: c.Tools.ImageGen, ImageAnalyze: c.Tools.ImageAnalyze,
			VideoGen: c.Tools.VideoGen, AudioGen: c.Tools.AudioGen,
			Browser: c.Tools.Browser, CodeExec: c.Tools.CodeExec,
			GitOps: c.Tools.GitOps, Cron: c.Tools.Cron,
			Notion: c.Tools.Notion, Calendar: c.Tools.Calendar,
			Weather: c.Tools.Weather, MCPDiscovery: c.Tools.MCPDiscovery,
			DiagramGen: c.Tools.DiagramGen,
		},
		Skills: yamlSkills{
			AutoInstall: c.Skills.AutoInstall, Marketplace: c.Skills.Marketplace,
			MarketplaceURL: c.Skills.MarketplaceURL, MarketplaceKey: "",
			LocalPath: c.Skills.LocalPath, AllowAutoUpdate: c.Skills.AllowAutoUpdate,
			MaxPerAgent: c.Skills.MaxPerAgent,
			ApprovedSkills: c.Skills.ApprovedSkills, BlockedKeywords: c.Skills.BlockedKeywords,
		},
		Logging: yamlLogging{
			Level: c.Logging.Level, Format: c.Logging.Format, Output: c.Logging.Output,
			MaxAge: dur(c.Logging.MaxAge), MaxSize: c.Logging.MaxSize,
			MaxBackups: c.Logging.MaxBackups, Compress: c.Logging.Compress,
			EnableTrace: c.Logging.EnableTrace,
		},
		UI: yamlUI{
			Theme: c.UI.Theme, SidebarCollapsed: c.UI.SidebarCollapsed,
			AutoRefreshSecs: c.UI.AutoRefreshSecs, ShowAnimations: c.UI.ShowAnimations,
			FontSize: c.UI.FontSize, CompactMode: c.UI.CompactMode, ColorAccent: c.UI.ColorAccent,
		},
		Pipelines: yamlPipelines{
			AutoStart: c.Pipelines.AutoStart, MaxConcurrent: c.Pipelines.MaxConcurrent,
			DefaultRetry: c.Pipelines.DefaultRetry,
		},
	}
}

func pd(p ProviderConfig) ProviderDef {
	return ProviderDef{Enabled: p.Enabled, APIKey: "", BaseURL: p.BaseURL, Model: p.Model}
}

func yamlMCPServers(c *Config) yamlMCP {
	servers := []yamlMCPServer{}
	for _, s := range c.MCP.Servers {
		servers = append(servers, yamlMCPServer{
			Name:       s.Name,
			Port:       s.Port,
			Enabled:    s.Enabled,
			Transport:  s.Transport,
			Command:    s.Command,
			Args:       s.Args,
			Env:        s.Env,
			ToolFilter: s.ToolFilter,
		})
	}
	return yamlMCP{
		Enabled: c.MCP.Enabled,
		Servers: servers,
	}
}