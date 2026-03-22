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
	Server       ServerConfig       `koanf:"server"`
	Database     DatabaseConfig     `koanf:"database"`
	Libraries    []LibraryConfig    `koanf:"libraries"`
	Transcoding  TranscodingConfig  `koanf:"transcoding"`
	Probe        ProbeConfig        `koanf:"probe"`
	Quality      QualityConfig      `koanf:"quality"`
	Auth         AuthConfig         `koanf:"auth"`
	Logging      LoggingConfig      `koanf:"logging"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	IdleTimeout     time.Duration `koanf:"idle_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	TLS             TLSConfig     `koanf:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled  bool   `koanf:"enabled"`
	CertPath string `koanf:"cert_path"`
	KeyPath  string `koanf:"key_path"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver           string        `koanf:"driver"`
	DSN              string        `koanf:"dsn"`
	MaxOpenConns     int           `koanf:"max_open_conns"`
	MaxIdleConns     int           `koanf:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `koanf:"conn_max_lifetime"`
}

// LibraryConfig holds media library configuration
type LibraryConfig struct {
	Name             string        `koanf:"name"`
	Path             string        `koanf:"path"`
	Type             string        `koanf:"type"`
	RefreshInterval  time.Duration `koanf:"refresh_interval"`
}

// TranscodingConfig holds transcoding configuration
type TranscodingConfig struct {
	Enabled      bool     `koanf:"enabled"`
	TempDir      string   `koanf:"temp_dir"`
	MaxConcurrent int      `koanf:"max_concurrent"`
	MaxBitrate    int64    `koanf:"max_bitrate"`
	VideoCodecs  []string `koanf:"video_codecs"`
	AudioCodecs  []string `koanf:"audio_codecs"`
	Containers   []string `koanf:"containers"`
}

// ProbeConfig holds device probing configuration
type ProbeConfig struct {
	Enabled       bool          `koanf:"enabled"`
	Timeout       time.Duration `koanf:"timeout"`
	MaxRetries    int           `koanf:"max_retries"`
	ScenarioDelay time.Duration `koanf:"scenario_delay"`
}

// QualityProfile represents a quality preset
type QualityProfile struct {
	Name       string `koanf:"name"`
	MaxWidth   int    `koanf:"max_width"`
	MaxHeight  int    `koanf:"max_height"`
	MaxBitrate int64  `koanf:"max_bitrate"`
}

// QualityConfig holds quality profile configuration
type QualityConfig struct {
	Profiles []QualityProfile `koanf:"profiles"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret      string        `koanf:"jwt_secret"`
	JWTExpiry      time.Duration `koanf:"jwt_expiry"`
	RefreshExpiry  time.Duration `koanf:"refresh_expiry"`
	APIKeyHeader   string        `koanf:"api_key_header"`
	RequireAuth    []string      `koanf:"require_auth"`
	PublicPaths    []string      `koanf:"public_paths"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level           string `koanf:"level"`
	Format          string `koanf:"format"`
	Output          string `koanf:"output"`
	IncludeTimestamp bool   `koanf:"include_timestamp"`
	IncludeCaller   bool   `koanf:"include_caller"`
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
		c.Server.Port = 8080
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
