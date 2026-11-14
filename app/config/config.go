// Package config loads and validates runtime configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Environment selects runtime behavior for the bot.
type Environment string

const (
	Production  Environment = "production"
	Development Environment = "development"
	Test        Environment = "test"
)

// Config contains validated runtime configuration. Its zero value preserves
// the production defaults, which also keeps tests and existing deployments
// safe when ENVIRONMENT is not set.
type Config struct {
	environment Environment
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	switch Environment(raw) {
	case "", Production:
		return Config{environment: Production}, nil
	case Development:
		return Config{environment: Development}, nil
	case Test:
		return Config{environment: Test}, nil
	default:
		return Config{}, fmt.Errorf("ENVIRONMENT must be production, development or test, got %q", raw)
	}
}

func (c Config) IsProduction() bool {
	return c.environment == "" || c.environment == Production
}

func (c Config) IsDevelopment() bool {
	return c.environment == Development
}

func (c Config) IsTest() bool {
	return c.environment == Test
}

// SpinCooldown keeps production behavior unchanged while making local and
// automated testing practical.
func (c Config) SpinCooldown() time.Duration {
	if c.IsDevelopment() || c.IsTest() {
		return 4 * time.Second
	}
	return 4 * time.Hour
}
