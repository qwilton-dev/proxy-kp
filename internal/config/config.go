package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	TLS         TLSConfig         `yaml:"tls"`
	Backends    []BackendConfig   `yaml:"backends"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Cache       CacheConfig       `yaml:"cache"`
	RateLimit   RateLimitConfig   `yaml:"rate_limit"`
	Logging     LoggingConfig     `yaml:"logging"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	Host         string        `yaml:"host"`
	HTTPPort     int           `yaml:"http_port"`
	HTTPSPort    int           `yaml:"https_port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type TLSConfig struct {
	Enabled bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type BackendConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

type HealthCheckConfig struct {
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	Endpoint         string        `yaml:"endpoint"`
	FailureThreshold int           `yaml:"failure_threshold"`
	RecoveryInterval time.Duration `yaml:"recovery_interval"`
}

type CacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
}

type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	Burst             int  `yaml:"burst"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	cfg.setDefaults()

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Host == "" {
		return fmt.Errorf("server host cannot be empty")
	}

	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}

	if c.Server.HTTPSPort <= 0 || c.Server.HTTPSPort > 65535 {
		return fmt.Errorf("invalid HTTPS port: %d", c.Server.HTTPSPort)
	}

	if c.TLS.Enabled && c.Server.HTTPPort == c.Server.HTTPSPort {
		return fmt.Errorf("HTTP and HTTPS ports must be different")
	}

	if len(c.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	for i, backend := range c.Backends {
		if backend.URL == "" {
			return fmt.Errorf("backend %d: URL cannot be empty", i)
		}
		if backend.Weight <= 0 {
			return fmt.Errorf("backend %d: weight must be positive", i)
		}
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert_file is required when TLS is enabled")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key_file is required when TLS is enabled")
		}
		if _, err := os.Stat(c.TLS.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file does not exist: %s", c.TLS.CertFile)
		}
		if _, err := os.Stat(c.TLS.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", c.TLS.KeyFile)
		}
	}

	if c.HealthCheck.Interval <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}
	if c.HealthCheck.Timeout <= 0 {
		return fmt.Errorf("health check timeout must be positive")
	}
	if c.HealthCheck.FailureThreshold <= 0 {
		return fmt.Errorf("health check failure threshold must be positive")
	}
	if c.HealthCheck.RecoveryInterval <= 0 {
		return fmt.Errorf("health check recovery interval must be positive")
	}

	if c.Cache.TTL < 0 {
		return fmt.Errorf("cache TTL cannot be negative")
	}

	if c.RateLimit.RequestsPerMinute <= 0 {
		return fmt.Errorf("rate limit requests per minute must be positive")
	}
	if c.RateLimit.Burst <= 0 {
		return fmt.Errorf("rate limit burst must be positive")
	}

	return nil
}

func (c *Config) setDefaults() {
	if c.Server.HTTPPort == 0 {
		c.Server.HTTPPort = 8080
	}
	if c.Server.HTTPSPort == 0 {
		c.Server.HTTPSPort = 8443
	}

	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 10 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 10 * time.Second
	}

	if c.HealthCheck.Interval == 0 {
		c.HealthCheck.Interval = 5 * time.Second
	}
	if c.HealthCheck.Timeout == 0 {
		c.HealthCheck.Timeout = 2 * time.Second
	}
	if c.HealthCheck.Endpoint == "" {
		c.HealthCheck.Endpoint = "/healthz"
	}
	if c.HealthCheck.FailureThreshold == 0 {
		c.HealthCheck.FailureThreshold = 3
	}
	if c.HealthCheck.RecoveryInterval == 0 {
		c.HealthCheck.RecoveryInterval = 15 * time.Second
	}

	if c.Cache.TTL == 0 {
		c.Cache.TTL = 60 * time.Second
	}

	if c.RateLimit.RequestsPerMinute == 0 {
		c.RateLimit.RequestsPerMinute = 600
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 100
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}
