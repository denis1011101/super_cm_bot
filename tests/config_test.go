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

func TestConfigLoadDevUser(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		id       string
		userName string
		wantID   int64
		wantName string
		wantErr  bool
	}{
		{name: "unset", env: "development"},
		{name: "default name", env: "development", id: "42", wantID: 42, wantName: "dev_user"},
		{name: "custom name strips @", env: "development", id: " 42 ", userName: "@denis", wantID: 42, wantName: "denis"},
		{name: "rejected in production", env: "production", id: "42", wantErr: true},
		{name: "rejected in test", env: "test", id: "42", wantErr: true},
		{name: "zero", env: "development", id: "0", wantErr: true},
		{name: "negative", env: "development", id: "-5", wantErr: true},
		{name: "not a number", env: "development", id: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tt.env)
			t.Setenv("DEV_FAKE_USERS", "")
			t.Setenv("DEV_USER_ID", tt.id)
			t.Setenv("DEV_USER_NAME", tt.userName)
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
			id, name, ok := cfg.DevUser()
			if ok != (tt.wantID != 0) || id != tt.wantID || name != tt.wantName {
				t.Errorf("DevUser() = (%d, %q, %v), want (%d, %q)", id, name, ok, tt.wantID, tt.wantName)
			}
		})
	}
}

func TestConfigLoadResetDB(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "unset", env: "development", value: "", want: false},
		{name: "true", env: "development", value: "true", want: true},
		{name: "false", env: "development", value: "false", want: false},
		{name: "1", env: "development", value: "1", want: true},
		{name: "false allowed in production", env: "production", value: "false", want: false},
		{name: "rejected in production", env: "production", value: "true", wantErr: true},
		{name: "rejected in test", env: "test", value: "true", wantErr: true},
		{name: "rejected by default environment", env: "", value: "true", wantErr: true},
		{name: "garbage", env: "development", value: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tt.env)
			t.Setenv("DEV_FAKE_USERS", "")
			t.Setenv("DEV_USER_ID", "")
			t.Setenv("DEV_RESET_DB", tt.value)
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
			if got := cfg.ResetDB(); got != tt.want {
				t.Errorf("ResetDB() = %v, want %v", got, tt.want)
			}
		})
	}
}
