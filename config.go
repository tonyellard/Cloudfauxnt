package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the CloudFauxnt configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Origins []Origin      `yaml:"origins"`
	CORS    CORSConfig    `yaml:"cors"`
	Signing SigningConfig `yaml:"signing"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port           int    `yaml:"port"`
	Host           string `yaml:"host"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// Origin represents a backend origin server
type Origin struct {
	Name             string   `yaml:"name"`
	URL              string   `yaml:"url"`
	PathPatterns     []string `yaml:"path_patterns"`
	StripPrefix      string   `yaml:"strip_prefix"`      // Optional: remove this prefix from request path
	TargetPrefix     string   `yaml:"target_prefix"`     // Optional: add this prefix to proxied path
	RequireSignature *bool    `yaml:"require_signature"` // Optional: require CloudFront signature for this origin (null/empty uses global setting)
}

// CORSConfig holds CORS policy settings
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"max_age"`
}

// SigningConfig holds CloudFront signing settings
type SigningConfig struct {
	Enabled       bool   `yaml:"enabled"`
	KeyPairID     string `yaml:"key_pair_id"`
	PublicKeyPath string `yaml:"public_key_path"`
	PublicKey     *rsa.PublicKey
	// Token options for testing and configuration
	TokenOptions TokenOptions `yaml:"token_options"`
}

// TokenOptions holds configuration for signed URL and cookie tokens
type TokenOptions struct {
	// ClockSkewSeconds allows for clock skew between client and server when validating expiration
	ClockSkewSeconds int `yaml:"clock_skew_seconds"`
	// DefaultURLTTLSeconds is the default TTL for signed URLs if not otherwise specified
	DefaultURLTTLSeconds int `yaml:"default_url_ttl_seconds"`
	// DefaultCookieTTLSeconds is the default TTL for signed cookies if not otherwise specified
	DefaultCookieTTLSeconds int `yaml:"default_cookie_ttl_seconds"`
	// AllowWildcardPatterns controls whether signed URLs can use wildcard patterns (default: false)
	AllowWildcardPatterns bool `yaml:"allow_wildcard_patterns"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Load public key if signing is enabled
	if config.Signing.Enabled {
		if err := config.loadPublicKey(); err != nil {
			return nil, fmt.Errorf("failed to load public key: %w", err)
		}
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d (must be 1-65535)", c.Server.Port)
	}
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.TimeoutSeconds <= 0 {
		c.Server.TimeoutSeconds = 30
	}

	// Validate origins
	if len(c.Origins) == 0 {
		return fmt.Errorf("at least one origin must be configured")
	}
	for i, origin := range c.Origins {
		if origin.Name == "" {
			return fmt.Errorf("origin %d: name is required", i)
		}
		if origin.URL == "" {
			return fmt.Errorf("origin %s: URL is required", origin.Name)
		}
		if len(origin.PathPatterns) == 0 {
			return fmt.Errorf("origin %s: at least one path pattern is required", origin.Name)
		}
	}

	// Validate CORS config
	if c.CORS.Enabled {
		if len(c.CORS.AllowedOrigins) == 0 {
			c.CORS.AllowedOrigins = []string{"*"}
		}
		if len(c.CORS.AllowedMethods) == 0 {
			c.CORS.AllowedMethods = []string{"GET", "HEAD", "OPTIONS"}
		}
		if len(c.CORS.AllowedHeaders) == 0 {
			c.CORS.AllowedHeaders = []string{"*"}
		}
		if c.CORS.MaxAge <= 0 {
			c.CORS.MaxAge = 3600
		}
	}

	// Validate signing config
	if c.Signing.Enabled {
		if c.Signing.KeyPairID == "" {
			return fmt.Errorf("signing.key_pair_id is required when signing is enabled")
		}
		if c.Signing.PublicKeyPath == "" {
			return fmt.Errorf("signing.public_key_path is required when signing is enabled")
		}
	}

	return nil
}

// loadPublicKey loads the RSA public key from the configured path
func (c *Config) loadPublicKey() error {
	keyData, err := os.ReadFile(c.Signing.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block from public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA")
	}

	c.Signing.PublicKey = rsaPub
	return nil
}

// FindOrigin returns the origin that matches the given path
func (c *Config) FindOrigin(path string) (*Origin, error) {
	// Match longest pattern first
	var bestMatch *Origin
	bestMatchLen := 0

	for i := range c.Origins {
		origin := &c.Origins[i]
		for _, pattern := range origin.PathPatterns {
			if matchPath(pattern, path) {
				patternLen := len(pattern)
				if patternLen > bestMatchLen {
					bestMatch = origin
					bestMatchLen = patternLen
				}
			}
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no origin found for path: %s", path)
	}

	return bestMatch, nil
}

// matchPath checks if a path matches a pattern (simple glob matching)
func matchPath(pattern, path string) bool {
	// Handle exact match
	if pattern == path {
		return true
	}

	// Handle wildcard patterns
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}

	// Handle prefix match
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}

	return false
}
