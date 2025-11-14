package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete agent configuration
type Config struct {
	DeviceID string      `mapstructure:"device_id"`
	NATS     NATSConfig  `mapstructure:"nats"`
	Tasks    TasksConfig `mapstructure:"tasks"`
	Commands CommandsConfig `mapstructure:"commands"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// NATSConfig holds NATS connection settings
type NATSConfig struct {
	URLs           []string      `mapstructure:"urls"`
	Auth           AuthConfig    `mapstructure:"auth"`
	MaxReconnects  int           `mapstructure:"max_reconnects"`
	ReconnectWait  time.Duration `mapstructure:"reconnect_wait"`
	DrainTimeout   time.Duration `mapstructure:"drain_timeout"`
}

// AuthConfig holds NATS authentication credentials
type AuthConfig struct {
	Type      string `mapstructure:"type"`       // creds, token, userpass
	CredsFile string `mapstructure:"creds_file"` // for creds auth
	Token     string `mapstructure:"token"`      // for token auth
	Username  string `mapstructure:"username"`   // for userpass auth
	Password  string `mapstructure:"password"`   // for userpass auth
}

// TasksConfig holds scheduled task configurations
type TasksConfig struct {
	Heartbeat     HeartbeatConfig     `mapstructure:"heartbeat"`
	SystemMetrics SystemMetricsConfig `mapstructure:"system_metrics"`
	ServiceCheck  ServiceCheckConfig  `mapstructure:"service_check"`
	Inventory     InventoryConfig     `mapstructure:"inventory"`
}

// HeartbeatConfig configures the heartbeat task
type HeartbeatConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

// SystemMetricsConfig configures metrics scraping
type SystemMetricsConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Interval     time.Duration `mapstructure:"interval"`
	ExporterURL  string        `mapstructure:"exporter_url"`
}

// ServiceCheckConfig configures service status monitoring
type ServiceCheckConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
	Services []string      `mapstructure:"services"`
}

// InventoryConfig configures system inventory reporting
type InventoryConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

// CommandsConfig holds command execution settings
type CommandsConfig struct {
	AllowedServices  []string `mapstructure:"allowed_services"`
	AllowedCommands  []string `mapstructure:"allowed_commands"`
	AllowedLogPaths  []string `mapstructure:"allowed_log_paths"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	v := viper.New()
	
	// Set config file path
	v.SetConfigFile(configPath)
	
	// Set defaults
	setDefaults(v)
	
	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	return &cfg, nil
}

// setDefaults sets sensible default values
func setDefaults(v *viper.Viper) {
	// NATS defaults
	v.SetDefault("nats.max_reconnects", -1) // infinite
	v.SetDefault("nats.reconnect_wait", "2s")
	v.SetDefault("nats.drain_timeout", "30s")
	
	// Task defaults
	v.SetDefault("tasks.heartbeat.enabled", true)
	v.SetDefault("tasks.heartbeat.interval", "1m")
	v.SetDefault("tasks.system_metrics.enabled", true)
	v.SetDefault("tasks.system_metrics.interval", "5m")
	v.SetDefault("tasks.system_metrics.exporter_url", "http://localhost:9182/metrics")
	v.SetDefault("tasks.service_check.enabled", true)
	v.SetDefault("tasks.service_check.interval", "1m")
	v.SetDefault("tasks.inventory.enabled", true)
	v.SetDefault("tasks.inventory.interval", "24h")
	
	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.file", "C:\\ProgramData\\WinAgent\\agent.log")
	v.SetDefault("logging.max_size_mb", 100)
	v.SetDefault("logging.max_backups", 3)
}

// validate checks that required fields are present and valid
func validate(cfg *Config) error {
	// Validate device_id
	if cfg.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	
	// Validate NATS URLs
	if len(cfg.NATS.URLs) == 0 {
		return fmt.Errorf("at least one NATS URL is required")
	}
	
	// Validate NATS auth
	switch cfg.NATS.Auth.Type {
	case "creds":
		if cfg.NATS.Auth.CredsFile == "" {
			return fmt.Errorf("creds_file is required for creds auth type")
		}
	case "token":
		if cfg.NATS.Auth.Token == "" {
			return fmt.Errorf("token is required for token auth type")
		}
	case "userpass":
		if cfg.NATS.Auth.Username == "" || cfg.NATS.Auth.Password == "" {
			return fmt.Errorf("username and password are required for userpass auth type")
		}
	case "none":
		// No validation needed
	default:
		return fmt.Errorf("invalid auth type: %s (must be creds, token, userpass, or none)", cfg.NATS.Auth.Type)
	}
	
	// Validate service check has services if enabled
	if cfg.Tasks.ServiceCheck.Enabled && len(cfg.Tasks.ServiceCheck.Services) == 0 {
		return fmt.Errorf("at least one service must be specified when service_check is enabled")
	}
	
	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", cfg.Logging.Level)
	}
	
	return nil
}
