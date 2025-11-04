package config

import (
	"fmt"
	"os"

	"github.com/lureiny/lookingglass/pkg/netutil"
	"gopkg.in/yaml.v3"
)

// Config represents the agent configuration
type Config struct {
	Agent    AgentConfig    `yaml:"agent"`
	Master   MasterConfig   `yaml:"master"`
	Executor ExecutorConfig `yaml:"executor"`
	Log      LogConfig      `yaml:"log"`
}

// AgentMetadata contains agent descriptive information
type AgentMetadata struct {
	Location    string `yaml:"location"`    // Geographic location (e.g., "Los Angeles", "Singapore")
	Provider    string `yaml:"provider"`    // Service provider (e.g., "AWS", "DigitalOcean", "Vultr")
	IDC         string `yaml:"idc"`         // Data center identifier (e.g., "us-west-1a", "sgp1")
	Description string `yaml:"description"` // Additional description
}

// AgentConfig contains agent-specific settings
type AgentConfig struct {
	ID            string        `yaml:"id"`
	Name          string        `yaml:"name"`
	IPv4          string        `yaml:"ipv4"`
	IPv6          string        `yaml:"ipv6"`
	HideIP        bool          `yaml:"hide_ip"`        // Whether to hide IP address (mask last 2 octets)
	GRPCPort      int           `yaml:"grpc_port"`      // DEPRECATED: No longer used in stream mode
	MaxConcurrent int           `yaml:"max_concurrent"` // Maximum concurrent tasks
	Metadata      AgentMetadata `yaml:"metadata"`       // Agent metadata (location, provider, etc.)
}

// MasterConfig contains master connection settings
type MasterConfig struct {
	Host              string `yaml:"host"`
	APIKey            string `yaml:"api_key"`
	TLSEnabled        bool   `yaml:"tls_enabled"`
	TLSCert           string `yaml:"tls_cert"`
	HeartbeatInterval int    `yaml:"heartbeat_interval"` // seconds
	RetryTimes        int    `yaml:"retry_times"`
	RetryInterval     int    `yaml:"retry_interval"` // seconds
}

// ExecutorType specifies the type of executor
type ExecutorType string

const (
	ExecutorTypeCommand ExecutorType = "command" // Execute external command
)

// ExecutorSpec defines how to execute a task
type ExecutorSpec struct {
	Type          ExecutorType `yaml:"type"`           // Executor type (command, http, etc.)
	Path          string       `yaml:"path"`           // Path to executable (for command type)
	DefaultArgs   []string     `yaml:"default_args"`   // Default arguments (used when no params from frontend)
	ArgsBuilder   string       `yaml:"args_builder"`   // Named args builder function (builtin, custom)
	LineFormatter string       `yaml:"line_formatter"` // Named line formatter function (none, newline)
}

// ConcurrencyConfig contains concurrency settings
type ConcurrencyConfig struct {
	Max int `yaml:"max"` // Max concurrent tasks for this specific task type
}

// TaskConfig defines configuration for a single task type
type TaskConfig struct {
	Enabled        *bool             `yaml:"enabled"`         // nil = use default, true/false = override
	DisplayName    string            `yaml:"display_name"`    // Display name for frontend
	RequiresTarget *bool             `yaml:"requires_target"` // Whether this task requires target parameter (nil = true)
	Executor       *ExecutorSpec     `yaml:"executor"`        // Executor specification (nil = use default)
	Concurrency    ConcurrencyConfig `yaml:"concurrency"`     // Concurrency settings
}

// ExecutorConfig contains executor settings
type ExecutorConfig struct {
	GlobalConcurrency int                    `yaml:"global_concurrency"` // Global max concurrent tasks (0 = use default)
	DefaultTimeout    int                    `yaml:"default_timeout"`    // seconds
	WorkDir           string                 `yaml:"work_dir"`
	Tasks             map[string]*TaskConfig `yaml:"tasks"` // Task configurations keyed by task name (ping, mtr, nexttrace, custom)
}

// LogConfig contains logging settings
type LogConfig struct {
	Level   string `yaml:"level"`
	File    string `yaml:"file"`
	Console bool   `yaml:"console"`
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// GetBuiltinTaskDefaults returns default configurations for builtin tasks
func GetBuiltinTaskDefaults() map[string]*TaskConfig {
	return map[string]*TaskConfig{
		"ping": {
			Enabled:     boolPtr(true),
			DisplayName: "Ping",
			Executor: &ExecutorSpec{
				Type:          ExecutorTypeCommand,
				Path:          "/usr/bin/ping",
				ArgsBuilder:   "builtin_ping",
				LineFormatter: "none",
			},
			Concurrency: ConcurrencyConfig{
				Max: 3, // Default: 3 concurrent ping tasks per agent
			},
		},
		"mtr": {
			Enabled:     boolPtr(true),
			DisplayName: "MTR",
			Executor: &ExecutorSpec{
				Type:          ExecutorTypeCommand,
				Path:          "/usr/bin/mtr",
				ArgsBuilder:   "builtin_mtr",
				LineFormatter: "none",
			},
			Concurrency: ConcurrencyConfig{
				Max: 2, // Default: 2 concurrent MTR tasks per agent
			},
		},
		"nexttrace": {
			Enabled:     boolPtr(true),
			DisplayName: "NextTrace",
			Executor: &ExecutorSpec{
				Type:          ExecutorTypeCommand,
				Path:          "/usr/bin/nexttrace",
				ArgsBuilder:   "builtin_nexttrace",
				LineFormatter: "newline",
			},
			Concurrency: ConcurrencyConfig{
				Max: 2, // Default: 2 concurrent nexttrace tasks per agent
			},
		},
	}
}

// MergeTaskConfig merges user config with builtin defaults
// User config takes precedence for all non-nil/non-zero fields
func MergeTaskConfig(userTask *TaskConfig, defaultTask *TaskConfig) *TaskConfig {
	if userTask == nil {
		return defaultTask
	}

	merged := &TaskConfig{}

	// Enabled: user value overrides, nil means use default
	if userTask.Enabled != nil {
		merged.Enabled = userTask.Enabled
	} else {
		merged.Enabled = defaultTask.Enabled
	}

	// DisplayName: non-empty user value overrides
	if userTask.DisplayName != "" {
		merged.DisplayName = userTask.DisplayName
	} else {
		merged.DisplayName = defaultTask.DisplayName
	}

	// Executor: merge executor specs if both exist, otherwise use whichever is present
	if userTask.Executor != nil && defaultTask.Executor != nil {
		merged.Executor = &ExecutorSpec{
			Type:          userTask.Executor.Type,
			Path:          userTask.Executor.Path,
			DefaultArgs:   userTask.Executor.DefaultArgs,
			ArgsBuilder:   userTask.Executor.ArgsBuilder,
			LineFormatter: userTask.Executor.LineFormatter,
		}
		// Fill in defaults for zero values
		if merged.Executor.Type == "" {
			merged.Executor.Type = defaultTask.Executor.Type
		}
		if merged.Executor.Path == "" {
			merged.Executor.Path = defaultTask.Executor.Path
		}
		if merged.Executor.ArgsBuilder == "" {
			merged.Executor.ArgsBuilder = defaultTask.Executor.ArgsBuilder
		}
		if merged.Executor.LineFormatter == "" {
			merged.Executor.LineFormatter = defaultTask.Executor.LineFormatter
		}
	} else if userTask.Executor != nil {
		merged.Executor = userTask.Executor
	} else {
		merged.Executor = defaultTask.Executor
	}

	// Concurrency: user value overrides if > 0
	if userTask.Concurrency.Max > 0 {
		merged.Concurrency.Max = userTask.Concurrency.Max
	} else {
		merged.Concurrency.Max = defaultTask.Concurrency.Max
	}

	return merged
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

	// Auto-detect IP addresses if not configured
	if err := cfg.autoDetectIPs(); err != nil {
		// Log warning but don't fail - IPs are optional
		fmt.Fprintf(os.Stderr, "Warning: failed to auto-detect IP addresses: %v\n", err)
	}

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for unspecified fields
func (c *Config) setDefaults() {
	// GRPCPort is deprecated - agents use stream mode and don't listen on any port

	if c.Agent.MaxConcurrent == 0 {
		c.Agent.MaxConcurrent = 5
	}

	if c.Master.HeartbeatInterval == 0 {
		c.Master.HeartbeatInterval = 30
	}

	if c.Master.RetryTimes == 0 {
		c.Master.RetryTimes = 3
	}

	if c.Master.RetryInterval == 0 {
		c.Master.RetryInterval = 5
	}

	if c.Executor.DefaultTimeout == 0 {
		c.Executor.DefaultTimeout = 300
	}

	if c.Executor.WorkDir == "" {
		c.Executor.WorkDir = "/tmp/lookingglass"
	}

	// Set global concurrency default
	if c.Executor.GlobalConcurrency == 0 {
		c.Executor.GlobalConcurrency = 10 // Default: 10 concurrent tasks globally
	}

	// Initialize Tasks map if nil
	if c.Executor.Tasks == nil {
		c.Executor.Tasks = make(map[string]*TaskConfig)
	}

	// Merge user task configs with builtin defaults
	builtinDefaults := GetBuiltinTaskDefaults()
	mergedTasks := make(map[string]*TaskConfig)

	// First, add all builtin tasks with defaults
	for taskName, defaultConfig := range builtinDefaults {
		userConfig := c.Executor.Tasks[taskName]
		mergedTasks[taskName] = MergeTaskConfig(userConfig, defaultConfig)
	}

	// Then, add any custom tasks that user defined but are not in builtins
	for taskName, userConfig := range c.Executor.Tasks {
		if _, isBuiltin := builtinDefaults[taskName]; !isBuiltin {
			// Custom task - use user config directly
			// Set defaults for required fields if not specified
			if userConfig.Executor == nil {
				userConfig.Executor = &ExecutorSpec{Type: ExecutorTypeCommand}
			}
			if userConfig.Concurrency.Max == 0 {
				userConfig.Concurrency.Max = 1 // Default: 1 concurrent task for custom tasks
			}
			mergedTasks[taskName] = userConfig
		}
	}

	// Replace with merged tasks
	c.Executor.Tasks = mergedTasks

	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	if c.Log.File == "" {
		c.Log.File = "logs/agent.log"
	}
}

// autoDetectIPs automatically detects and fills in IPv4 and IPv6 addresses if not configured
func (c *Config) autoDetectIPs() error {
	// Auto-detect IPv4 if not configured
	if c.Agent.IPv4 == "" {
		fmt.Println("IPv4 not configured, attempting auto-detection...")
		ipv4, err := netutil.GetPublicIPv4()
		if err == nil && ipv4 != "" {
			c.Agent.IPv4 = ipv4
			fmt.Printf("Auto-detected IPv4: %s\n", ipv4)
		} else {
			fmt.Printf("Failed to auto-detect IPv4: %v\n", err)
		}
	}

	// Auto-detect IPv6 if not configured
	if c.Agent.IPv6 == "" {
		fmt.Println("IPv6 not configured, attempting auto-detection...")
		ipv6, err := netutil.GetPublicIPv6()
		if err == nil && ipv6 != "" {
			c.Agent.IPv6 = ipv6
			fmt.Printf("Auto-detected IPv6: %s\n", ipv6)
		} else {
			// IPv6 is optional, just log the failure
			fmt.Printf("Failed to auto-detect IPv6 (this is normal if IPv6 is not available): %v\n", err)
		}
	}

	return nil
}

// validate validates the configuration
func (c *Config) validate() error {
	if c.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}

	if c.Agent.Name == "" {
		return fmt.Errorf("agent.name is required")
	}

	if c.Master.Host == "" {
		return fmt.Errorf("master.host is required")
	}

	if c.Master.APIKey == "" {
		return fmt.Errorf("master.api_key is required")
	}

	if c.Agent.MaxConcurrent < 1 {
		return fmt.Errorf("agent.max_concurrent must be at least 1")
	}

	return nil
}
