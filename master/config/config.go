package config

import (
	"fmt"
	"os"

	pb "github.com/lureiny/lookingglass/pb"
	"gopkg.in/yaml.v3"
)

// Config represents the master configuration
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Auth         AuthConfig         `yaml:"auth"`
	Concurrency  ConcurrencyConfig  `yaml:"concurrency"`
	Agent        AgentConfig        `yaml:"agent"`
	Task         TaskConfig         `yaml:"task"`
	Notification NotificationConfig `yaml:"notification"`
	Log          LogConfig          `yaml:"log"`
	Branding     BrandingConfig     `yaml:"branding"`
}

// ServerConfig contains server settings
type ServerConfig struct {
	GRPCPort int `yaml:"grpc_port"`
	WSPort   int `yaml:"ws_port"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Mode        string   `yaml:"mode"` // "api_key" or "ip_whitelist"
	APIKey      string   `yaml:"api_key"`
	IPWhitelist []string `yaml:"ip_whitelist"`
}

// ConcurrencyConfig contains concurrency settings
type ConcurrencyConfig struct {
	GlobalMax       int `yaml:"global_max"`
	AgentDefaultMax int `yaml:"agent_default_max"`
}

// AgentConfig contains agent management settings
type AgentConfig struct {
	HeartbeatTimeout     int `yaml:"heartbeat_timeout"`      // seconds
	HeartbeatInterval    int `yaml:"heartbeat_interval"`     // seconds
	OfflineCheckInterval int `yaml:"offline_check_interval"` // seconds
}

// TaskConfig contains task management settings
type TaskConfig struct {
	DefaultTimeout   int `yaml:"default_timeout"`    // seconds
	HistoryRetention int `yaml:"history_retention"`  // hours
	DefaultPingCount int `yaml:"default_ping_count"` // default ping count
	DefaultMTRCount  int `yaml:"default_mtr_count"`  // default mtr count
}

// NotificationConfig contains notification settings
type NotificationConfig struct {
	Enabled bool                `yaml:"enabled"`
	Events  NotificationEvents  `yaml:"events"`
	Bark    *BarkNotifierConfig `yaml:"bark,omitempty"`
	// Future notifiers can be added here:
	// Telegram *TelegramConfig `yaml:"telegram,omitempty"`
	// Feishu   *FeishuConfig   `yaml:"feishu,omitempty"`
	// Dingtalk *DingtalkConfig `yaml:"dingtalk,omitempty"`
	// Ntfy     *NtfyConfig     `yaml:"ntfy,omitempty"`
}

// NotificationEvents controls which events trigger notifications
type NotificationEvents struct {
	AgentOnline  bool `yaml:"agent_online"`
	AgentOffline bool `yaml:"agent_offline"`
	AgentError   bool `yaml:"agent_error"`
	TaskFailed   bool `yaml:"task_failed"`
}

// BarkNotifierConfig contains Bark-specific configuration
type BarkNotifierConfig struct {
	ServerURL string `yaml:"server_url"` // Full Bark URL
	DeviceKey string `yaml:"device_key"` // Or just device key
	Sound     string `yaml:"sound"`      // Notification sound
	Icon      string `yaml:"icon"`       // Icon URL
	Group     string `yaml:"group"`      // Notification group
}

// LogConfig contains logging settings
type LogConfig struct {
	Level   string `yaml:"level"`
	File    string `yaml:"file"`
	Console bool   `yaml:"console"`
}

// BrandingConfig contains branding customization settings
type BrandingConfig struct {
	SiteTitle  string `yaml:"site_title"`  // Website title
	LogoURL    string `yaml:"logo_url"`    // Logo image URL (optional)
	LogoText   string `yaml:"logo_text"`   // Logo text in header (optional)
	Subtitle   string `yaml:"subtitle"`    // Subtitle text below logo (optional)
	FooterText string `yaml:"footer_text"` // Custom footer (supports HTML)
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for unspecified fields
func (c *Config) setDefaults() {
	if c.Server.GRPCPort == 0 {
		c.Server.GRPCPort = 50051
	}

	if c.Server.WSPort == 0 {
		c.Server.WSPort = 8080
	}

	if c.Concurrency.GlobalMax == 0 {
		c.Concurrency.GlobalMax = 50
	}

	if c.Concurrency.AgentDefaultMax == 0 {
		c.Concurrency.AgentDefaultMax = 5
	}

	if c.Agent.HeartbeatTimeout == 0 {
		c.Agent.HeartbeatTimeout = 60
	}

	if c.Agent.HeartbeatInterval == 0 {
		c.Agent.HeartbeatInterval = 30
	}

	if c.Agent.OfflineCheckInterval == 0 {
		c.Agent.OfflineCheckInterval = 60
	}

	if c.Task.DefaultTimeout == 0 {
		c.Task.DefaultTimeout = 300
	}

	if c.Task.HistoryRetention == 0 {
		c.Task.HistoryRetention = 24
	}

	if c.Task.DefaultPingCount == 0 {
		c.Task.DefaultPingCount = 4
	}

	if c.Task.DefaultMTRCount == 0 {
		c.Task.DefaultMTRCount = 4
	}

	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	if c.Log.File == "" {
		c.Log.File = "logs/master.log"
	}

	if c.Branding.SiteTitle == "" {
		c.Branding.SiteTitle = "LookingGlass - Network Diagnostics"
	}

	// Branding default value initialization rules:
	// 1. If logo_url is set, use configured values for logo_text and subtitle (can be empty)
	// 2. If logo_url is not set but logo_text is set, use configured subtitle (can be empty)
	// 3. If neither logo_url nor logo_text is set, initialize both logo_text and subtitle to defaults
	if c.Branding.LogoURL == "" && c.Branding.LogoText == "" {
		// Case 3: No image, no text - initialize both to defaults
		c.Branding.LogoText = "🔍 LookingGlass"
		if c.Branding.Subtitle == "" {
			c.Branding.Subtitle = "Network Diagnostics Platform"
		}
	}
	// Case 1 and 2: Use configured values (no additional initialization needed)
}

// validate validates the configuration
func (c *Config) validate() error {
	// Validate auth mode
	if c.Auth.Mode != "api_key" && c.Auth.Mode != "ip_whitelist" {
		return fmt.Errorf("auth.mode must be 'api_key' or 'ip_whitelist'")
	}

	if c.Auth.APIKey == "" {
		return fmt.Errorf("auth.api_key is required")
	}

	if c.Auth.Mode == "ip_whitelist" && len(c.Auth.IPWhitelist) == 0 {
		return fmt.Errorf("auth.ip_whitelist cannot be empty when mode is 'ip_whitelist'")
	}

	if c.Concurrency.GlobalMax < 1 {
		return fmt.Errorf("concurrency.global_max must be at least 1")
	}

	if c.Concurrency.AgentDefaultMax < 1 {
		return fmt.Errorf("concurrency.agent_default_max must be at least 1")
	}

	return nil
}

// GetAuthMode returns the protobuf auth mode enum
func (c *Config) GetAuthMode() pb.AuthMode {
	switch c.Auth.Mode {
	case "api_key":
		return pb.AuthMode_AUTH_MODE_API_KEY
	case "ip_whitelist":
		return pb.AuthMode_AUTH_MODE_IP_WHITELIST
	default:
		return pb.AuthMode_AUTH_MODE_UNSPECIFIED
	}
}
