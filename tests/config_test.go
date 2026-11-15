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
			t.Setenv("DEV_FAKE_USERS", "")
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
	t.Setenv("DEV_FAKE_USERS", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() should reject an unknown environment")
	}
}

func TestConfigLoadFakeUsers(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		value   string
		want    int
		wantErr bool
	}{
		{name: "unset", env: "development", value: "", want: 0},
		{name: "development count", env: "development", value: "5", want: 5},
		{name: "trims spaces", env: "development", value: " 7 ", want: 7},
		{name: "max", env: "development", value: "100", want: 100},
		{name: "zero is allowed in production", env: "production", value: "0", want: 0},
		{name: "rejected in production", env: "production", value: "3", wantErr: true},
		{name: "rejected in test", env: "test", value: "3", wantErr: true},
		{name: "rejected by default environment", env: "", value: "3", wantErr: true},
		{name: "negative", env: "development", value: "-1", wantErr: true},
		{name: "too many", env: "development", value: "101", wantErr: true},
		{name: "not a number", env: "development", value: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tt.env)
			t.Setenv("DEV_FAKE_USERS", tt.value)
			cfg, err := config.Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() should fail")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := cfg.FakeUserCount(); got != tt.want {
				t.Errorf("FakeUserCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
