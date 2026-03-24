// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/tenkile/tenkile/pkg/codec"
	"github.com/zeebo/xxh3"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Database     DatabaseConfig     `yaml:"database"`
	Libraries    []LibraryConfig    `yaml:"libraries"`
	Transcoding  TranscodingConfig  `yaml:"transcoding"`
	Probe        ProbeConfig        `yaml:"probe"`
	Quality      QualityConfig      `yaml:"quality"`
	Auth         AuthConfig         `yaml:"auth"`
	Logging      LoggingConfig      `yaml:"logging"`
	Security     SecurityConfig     `yaml:"security"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	// Enable/disable security headers
	EnableHeaders bool `yaml:"enable_headers"`
	
	// Content-Security-Policy
	CSP string `yaml:"csp"`
	
	// HSTS (Strict-Transport-Security)
	HSTSEnabled bool `yaml:"hsts_enabled"`
	HSTSMaxAge  int  `yaml:"hsts_max_age"` // in seconds
	
	// X-Frame-Options
	XFrameOptions string `yaml:"x_frame_options"` // "DENY", "SAMEORIGIN", or empty to disable
	
	// X-Content-Type-Options
	XContentTypeOptions string `yaml:"x_content_type_options"` // "nosniff" or empty to disable
	
	// X-XSS-Protection (deprecated but included for older browser compatibility)
	XXSSProtection string `yaml:"x_xss_protection"` // "1; mode=block" or empty to disable
	
	// Referrer-Policy
	ReferrerPolicy string `yaml:"referrer_policy"` // "strict-origin-when-cross-origin", "no-referrer", etc.
	
	// Permissions-Policy
	PermissionsPolicy string `yaml:"permissions_policy"` // Feature policy
	
	// Custom additional headers (key: value pairs)
	AdditionalHeaders map[string]string `yaml:"additional_headers"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	TLS             TLSConfig     `yaml:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver           string        `yaml:"driver"`
	DSN              string        `yaml:"dsn"`
	MaxOpenConns     int           `yaml:"max_open_conns"`
	MaxIdleConns     int           `yaml:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `yaml:"conn_max_lifetime"`
}

// LibraryConfig holds media library configuration
type LibraryConfig struct {
	Name             string        `yaml:"name"`
	Path             string        `yaml:"path"`
	Type             string        `yaml:"type"`
	RefreshInterval  time.Duration `yaml:"refresh_interval"`
}

// TranscodingConfig holds transcoding configuration
type TranscodingConfig struct {
	Enabled      bool     `yaml:"enabled"`
	TempDir      string   `yaml:"temp_dir"`
	MaxConcurrent int      `yaml:"max_concurrent"`
	MaxBitrate    int64    `yaml:"max_bitrate"`
	VideoCodecs  []string `yaml:"video_codecs"`
	AudioCodecs  []string `yaml:"audio_codecs"`
	Containers   []string `yaml:"containers"`
}

// ProbeConfig holds device probing configuration
type ProbeConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Timeout       time.Duration `yaml:"timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	ScenarioDelay time.Duration `yaml:"scenario_delay"`
}

// QualityProfile represents a quality preset
type QualityProfile struct {
	Name       string `yaml:"name"`
	MaxWidth   int    `yaml:"max_width"`
	MaxHeight  int    `yaml:"max_height"`
	MaxBitrate int64  `yaml:"max_bitrate"`
}

// QualityConfig holds quality profile configuration
type QualityConfig struct {
	Profiles []QualityProfile `yaml:"profiles"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret      string   `yaml:"jwt_secret"`
	APIKey         string   `yaml:"api_key"`
	JWTExpiry      time.Duration `yaml:"jwt_expiry"`
	RefreshExpiry  time.Duration `yaml:"refresh_expiry"`
	APIKeyHeader   string   `yaml:"api_key_header"`
	RequireAuth    []string `yaml:"require_auth"`
	PublicPaths    []string `yaml:"public_paths"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	ProductionMode bool     `yaml:"production_mode"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level           string `yaml:"level"`
	Format          string `yaml:"format"`
	Output          string `yaml:"output"`
	IncludeTimestamp bool   `yaml:"include_timestamp"`
	IncludeCaller   bool   `yaml:"include_caller"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults
	cfg.applyDefaults()

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8765
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 120 * time.Second
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 30 * time.Second
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.DSN == "" {
		c.Database.DSN = "./data/tenkile.db"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 25
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.Auth.APIKeyHeader == "" {
		c.Auth.APIKeyHeader = "X-API-Key"
	}
	if c.Auth.JWTExpiry == 0 {
		c.Auth.JWTExpiry = time.Hour * 24
	}
	if c.Auth.RefreshExpiry == 0 {
		c.Auth.RefreshExpiry = time.Hour * 24 * 7
	}
	// Set CORS defaults based on production mode
	if len(c.Auth.AllowedOrigins) == 0 {
		if c.Auth.ProductionMode {
			// Production: require explicit origins, default to empty (strict)
			c.Auth.AllowedOrigins = []string{}
		} else {
			// Development: allow localhost
			c.Auth.AllowedOrigins = []string{"http://localhost:*", "http://127.0.0.1:*"}
		}
	}
	
	// Apply security defaults
	c.applySecurityDefaults()
}

func (c *Config) applySecurityDefaults() {
	// Security headers enabled by default (unless explicitly disabled)
	// Default CSP for media server
	if c.Security.CSP == "" {
		if c.Auth.ProductionMode {
			c.Security.CSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self'; connect-src 'self'; frame-ancestors 'none';"
		} else {
			// Development: more permissive
			c.Security.CSP = "default-src 'self' 'unsafe-inline' 'unsafe-eval'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self' ws://localhost:* http://localhost:*; frame-ancestors 'self';"
		}
	}
	
	// HSTS defaults
	if c.Security.HSTSMaxAge == 0 {
		if c.Auth.ProductionMode {
			c.Security.HSTSMaxAge = 31536000 // 1 year
		} else {
			c.Security.HSTSMaxAge = 86400 // 1 day for dev
		}
	}
	
	// X-Frame-Options
	if c.Security.XFrameOptions == "" {
		if c.Auth.ProductionMode {
			c.Security.XFrameOptions = "DENY"
		} else {
			c.Security.XFrameOptions = "SAMEORIGIN"
		}
	}
	
	// X-Content-Type-Options
	if c.Security.XContentTypeOptions == "" {
		c.Security.XContentTypeOptions = "nosniff"
	}
	
	// X-XSS-Protection (deprecated but kept for older browsers)
	if c.Security.XXSSProtection == "" {
		c.Security.XXSSProtection = "1; mode=block"
	}
	
	// Referrer-Policy
	if c.Security.ReferrerPolicy == "" {
		if c.Auth.ProductionMode {
			c.Security.ReferrerPolicy = "strict-origin-when-cross-origin"
		} else {
			c.Security.ReferrerPolicy = "no-referrer-when-downgrade"
		}
	}
	
	// Permissions-Policy
	if c.Security.PermissionsPolicy == "" {
		c.Security.PermissionsPolicy = "geolocation=(), microphone=(), camera=(), payment=()"
	}
}

// GenerateJWTSecret generates a random JWT secret
func GenerateJWTSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// HashDeviceID creates a consistent hash for device identification
func HashDeviceID(deviceID string) string {
	h := xxh3.New()
	h.Write([]byte(deviceID))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// GetCodecInfo returns codec information for a given codec name
func GetCodecInfo(name string) (*codec.Codec, bool) {
	return codec.GetByName(name)
}

// ValidateCodec checks if a codec is supported
func ValidateCodec(name string) bool {
	_, ok := codec.GetByName(name)
	return ok
}
