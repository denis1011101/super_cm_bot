package tests

import (
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app/config"
)

func TestConfigLoadEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		environment config.Environment
		cooldown    time.Duration
	}{
		{name: "default is production", value: "", environment: config.Production, cooldown: 4 * time.Hour},
		{name: "production", value: "production", environment: config.Production, cooldown: 4 * time.Hour},
		{name: "development", value: "development", environment: config.Development, cooldown: 4 * time.Second},
		{name: "test", value: "test", environment: config.Test, cooldown: 4 * time.Second},
		{name: "normalizes value", value: " Development ", environment: config.Development, cooldown: 4 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tt.value)
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if got := cfg.IsProduction(); got != (tt.environment == config.Production) {
				t.Errorf("IsProduction() = %v", got)
			}
			if got := cfg.IsDevelopment(); got != (tt.environment == config.Development) {
				t.Errorf("IsDevelopment() = %v", got)
			}
			if got := cfg.IsTest(); got != (tt.environment == config.Test) {
				t.Errorf("IsTest() = %v", got)
			}
			if got := cfg.SpinCooldown(); got != tt.cooldown {
				t.Errorf("SpinCooldown() = %v, want %v", got, tt.cooldown)
			}
		})
	}
}

func TestConfigLoadRejectsUnknownEnvironment(t *testing.T) {
	t.Setenv("ENVIRONMENT", "staging")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() should reject an unknown environment")
	}
}
