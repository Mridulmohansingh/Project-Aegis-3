// Package config provides hierarchical configuration management for AEGIS services.
//
// Configuration is loaded from YAML files with environment variable overrides.
// Environment variables follow the pattern AEGIS_SECTION_KEY (e.g., AEGIS_DATABASE_HOST).
//
// Usage:
//
//	cfg, err := config.Load("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(cfg.Server.Port)
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration for an AEGIS service.
type Config struct {
	Server        ServerConfig        `mapstructure:"server" json:"server"`
	Database      DatabaseConfig      `mapstructure:"database" json:"database"`
	Redis         RedisConfig         `mapstructure:"redis" json:"redis"`
	Kafka         KafkaConfig         `mapstructure:"kafka" json:"kafka"`
	Auth          AuthConfig          `mapstructure:"auth" json:"auth"`
	Vault         VaultConfig         `mapstructure:"vault" json:"vault"`
	Observability ObservabilityConfig `mapstructure:"observability" json:"observability"`
	Logging       LoggingConfig       `mapstructure:"logging" json:"logging"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string        `mapstructure:"host" json:"host"`
	Port            int           `mapstructure:"port" json:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" json:"shutdown_timeout"`
	MaxRequestSize  int64         `mapstructure:"max_request_size" json:"max_request_size"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host" json:"host"`
	Port            int           `mapstructure:"port" json:"port"`
	User            string        `mapstructure:"user" json:"user"`
	Password        string        `mapstructure:"password" json:"-"` // Never log passwords
	DBName          string        `mapstructure:"dbname" json:"dbname"`
	SSLMode         string        `mapstructure:"ssl_mode" json:"ssl_mode"`
	MaxConns        int32         `mapstructure:"max_conns" json:"max_conns"`
	MinConns        int32         `mapstructure:"min_conns" json:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime" json:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time" json:"max_conn_idle_time"`
	MigrationsPath  string        `mapstructure:"migrations_path" json:"migrations_path"`
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addresses    []string      `mapstructure:"addresses" json:"addresses"`
	Password     string        `mapstructure:"password" json:"-"`
	DB           int           `mapstructure:"db" json:"db"`
	PoolSize     int           `mapstructure:"pool_size" json:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
}

// KafkaConfig holds Apache Kafka connection settings.
type KafkaConfig struct {
	Brokers          []string `mapstructure:"brokers" json:"brokers"`
	GroupID          string   `mapstructure:"group_id" json:"group_id"`
	SecurityProtocol string   `mapstructure:"security_protocol" json:"security_protocol"`
	SASLMechanism    string   `mapstructure:"sasl_mechanism" json:"sasl_mechanism"`
	SASLUsername     string   `mapstructure:"sasl_username" json:"sasl_username"`
	SASLPassword     string   `mapstructure:"sasl_password" json:"-"`
}

// AuthConfig holds authentication and authorization settings.
type AuthConfig struct {
	JWTIssuer       string        `mapstructure:"jwt_issuer" json:"jwt_issuer"`
	JWTAudience     string        `mapstructure:"jwt_audience" json:"jwt_audience"`
	JWKSUrl         string        `mapstructure:"jwks_url" json:"jwks_url"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl" json:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl" json:"refresh_token_ttl"`
	OPAEndpoint     string        `mapstructure:"opa_endpoint" json:"opa_endpoint"`
}

// VaultConfig holds HashiCorp Vault settings.
type VaultConfig struct {
	Address   string `mapstructure:"address" json:"address"`
	Token     string `mapstructure:"token" json:"-"`
	MountPath string `mapstructure:"mount_path" json:"mount_path"`
	Namespace string `mapstructure:"namespace" json:"namespace"`
}

// ObservabilityConfig holds metrics and tracing settings.
type ObservabilityConfig struct {
	MetricsPort    int    `mapstructure:"metrics_port" json:"metrics_port"`
	MetricsPath    string `mapstructure:"metrics_path" json:"metrics_path"`
	TracingEnabled bool   `mapstructure:"tracing_enabled" json:"tracing_enabled"`
	OTLPEndpoint   string `mapstructure:"otlp_endpoint" json:"otlp_endpoint"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level       string `mapstructure:"level" json:"level"`
	Environment string `mapstructure:"environment" json:"environment"`
}

// Load reads configuration from the given file path with environment variable overrides.
// Environment variables are prefixed with AEGIS_ and use underscores as separators.
// For example, AEGIS_DATABASE_HOST overrides the database.host config key.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from file
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
		}
	}

	// Environment variable overrides
	v.SetEnvPrefix("AEGIS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets sensible default values for all configuration keys.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.idle_timeout", 120*time.Second)
	v.SetDefault("server.shutdown_timeout", 15*time.Second)
	v.SetDefault("server.max_request_size", 10*1024*1024) // 10MB

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "aegis")
	v.SetDefault("database.dbname", "aegis")
	v.SetDefault("database.ssl_mode", "require")
	v.SetDefault("database.max_conns", 25)
	v.SetDefault("database.min_conns", 5)
	v.SetDefault("database.max_conn_lifetime", 1*time.Hour)
	v.SetDefault("database.max_conn_idle_time", 30*time.Minute)

	// Redis defaults
	v.SetDefault("redis.addresses", []string{"localhost:6379"})
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 50)
	v.SetDefault("redis.min_idle_conns", 10)
	v.SetDefault("redis.dial_timeout", 5*time.Second)
	v.SetDefault("redis.read_timeout", 3*time.Second)
	v.SetDefault("redis.write_timeout", 3*time.Second)

	// Auth defaults
	v.SetDefault("auth.access_token_ttl", 15*time.Minute)
	v.SetDefault("auth.refresh_token_ttl", 24*time.Hour)

	// Observability defaults
	v.SetDefault("observability.metrics_port", 9090)
	v.SetDefault("observability.metrics_path", "/metrics")
	v.SetDefault("observability.tracing_enabled", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.environment", "production")
}

// Validate checks that required configuration values are present and valid.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("database.dbname is required")
	}
	if c.Database.MaxConns < 1 {
		return fmt.Errorf("database.max_conns must be at least 1")
	}
	if c.Database.MinConns < 0 {
		return fmt.Errorf("database.min_conns must be non-negative")
	}
	if c.Database.MinConns > c.Database.MaxConns {
		return fmt.Errorf("database.min_conns (%d) cannot exceed max_conns (%d)", c.Database.MinConns, c.Database.MaxConns)
	}
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server.read_timeout must be positive")
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("server.write_timeout must be positive")
	}
	return nil
}
