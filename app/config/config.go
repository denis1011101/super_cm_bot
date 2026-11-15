// Package config loads and validates runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
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
	environment   Environment
	fakeUserCount int
	devUserID     int64
	devUserName   string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	cfg := Config{}
	switch Environment(raw) {
	case "", Production:
		cfg.environment = Production
	case Development:
		cfg.environment = Development
	case Test:
		cfg.environment = Test
	default:
		return Config{}, fmt.Errorf("ENVIRONMENT must be production, development or test, got %q", raw)
	}

	if rawCount := strings.TrimSpace(os.Getenv("DEV_FAKE_USERS")); rawCount != "" {
		count, err := strconv.Atoi(rawCount)
		if err != nil || count < 0 || count > 100 {
			return Config{}, fmt.Errorf("DEV_FAKE_USERS must be an integer from 0 to 100, got %q", rawCount)
		}
		if count > 0 && !cfg.IsDevelopment() {
			return Config{}, fmt.Errorf("DEV_FAKE_USERS is allowed only when ENVIRONMENT=development")
		}
		cfg.fakeUserCount = count
	}

	if rawID := strings.TrimSpace(os.Getenv("DEV_USER_ID")); rawID != "" {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			return Config{}, fmt.Errorf("DEV_USER_ID must be a positive Telegram user id, got %q", rawID)
		}
		if !cfg.IsDevelopment() {
			return Config{}, fmt.Errorf("DEV_USER_ID is allowed only when ENVIRONMENT=development")
		}
		cfg.devUserID = id
		cfg.devUserName = strings.TrimPrefix(strings.TrimSpace(os.Getenv("DEV_USER_NAME")), "@")
		if cfg.devUserName == "" {
			cfg.devUserName = "dev_user"
		}
	}
	return cfg, nil
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

// FakeUserCount is the number of deterministic test users to ensure in the
// development database. Zero disables test user creation.
func (c Config) FakeUserCount() int {
	return c.fakeUserCount
}

// DevUser returns the real Telegram user to register at startup in the
// development database. ok is false when DEV_USER_ID is not set.
func (c Config) DevUser() (id int64, name string, ok bool) {
	return c.devUserID, c.devUserName, c.devUserID != 0
}

// SpinCooldown keeps production behavior unchanged while making local and
// automated testing practical.
func (c Config) SpinCooldown() time.Duration {
	if c.IsDevelopment() || c.IsTest() {
		return 4 * time.Second
	}
	return 4 * time.Hour
}
