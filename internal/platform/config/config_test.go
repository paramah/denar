package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var allKeys = []string{
	"DENAR_ADDR", "DENAR_DB", "DENAR_ADMIN_EMAIL", "DENAR_ADMIN_PASSWORD", "DENAR_ADMIN_PASSWORD_FILE",
	"DENAR_ADMIN_NAME", "DENAR_LOCAL_LOGIN", "DENAR_BASE_URL", "DENAR_SESSION_TTL", "DENAR_INVITE_TTL",
	"DENAR_OIDC_ISSUER", "DENAR_OIDC_CLIENT_ID", "DENAR_OIDC_CLIENT_SECRET", "DENAR_OIDC_AUTO_CREATE",
	"DENAR_AUTO_MIGRATE", "DENAR_TRUSTED_PROXIES", "DENAR_INSTANCE_NAME", "DENAR_BOT_RATE",
}

// clean unsets every variable for the test.
func clean(t *testing.T) {
	t.Helper()
	for _, k := range allKeys {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestDefaults(t *testing.T) {
	clean(t)
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.DatabasePath != "denar.db" || !cfg.LocalLogin || cfg.AutoMigrate ||
		cfg.SessionTTL != 168*time.Hour || cfg.InviteTTL != 168*time.Hour || cfg.BaseURL != "http://localhost:8080" ||
		cfg.InstanceName != "Denar" || cfg.BotRate != 120 || cfg.OIDCAutoCreate || cfg.HasAdmin() || cfg.OIDCEnabled() {
		t.Errorf("defaults = %+v", cfg)
	}
}

func TestValues(t *testing.T) {
	clean(t)
	t.Setenv("DENAR_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("DENAR_ADMIN_PASSWORD", "a long admin password")
	t.Setenv("DENAR_BASE_URL", "https://denar.example.com/")
	t.Setenv("DENAR_OIDC_ISSUER", "https://id.example.com")
	t.Setenv("DENAR_OIDC_CLIENT_ID", "denar")
	t.Setenv("DENAR_OIDC_CLIENT_SECRET", "s3cret")
	t.Setenv("DENAR_OIDC_AUTO_CREATE", "true")
	t.Setenv("DENAR_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.1.1")
	t.Setenv("DENAR_INVITE_TTL", "48h")
	t.Setenv("DENAR_BOT_RATE", "60")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HasAdmin() || !cfg.OIDCEnabled() || !cfg.OIDCAutoCreate || cfg.BaseURL != "https://denar.example.com" ||
		len(cfg.TrustedProxies) != 2 || cfg.InviteTTL != 48*time.Hour || cfg.BotRate != 60 {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"DENAR_ADMIN_EMAIL": "a@b.c"}, "DENAR_ADMIN_PASSWORD must be set"},
		{map[string]string{"DENAR_ADMIN_PASSWORD": "a long admin password"}, "DENAR_ADMIN_EMAIL must be set"},
		{map[string]string{"DENAR_ADMIN_EMAIL": "a@b.c", "DENAR_ADMIN_PASSWORD": "short"}, "at least 12"},
		{map[string]string{"DENAR_LOCAL_LOGIN": "maybe"}, "DENAR_LOCAL_LOGIN must be true or false"},
		{map[string]string{"DENAR_SESSION_TTL": "-1h"}, "DENAR_SESSION_TTL must be a positive duration"},
		{map[string]string{"DENAR_INVITE_TTL": "x"}, "DENAR_INVITE_TTL must be a positive duration"},
		{map[string]string{"DENAR_BASE_URL": "ftp://x"}, "DENAR_BASE_URL must be an origin"},
		{map[string]string{"DENAR_OIDC_ISSUER": "https://id", "DENAR_OIDC_CLIENT_ID": "c", "DENAR_OIDC_CLIENT_SECRET": "s"}, "DENAR_BASE_URL must be set"},
		{map[string]string{"DENAR_TRUSTED_PROXIES": "nope"}, "DENAR_TRUSTED_PROXIES must list networks"},
		{map[string]string{"DENAR_INSTANCE_NAME": strings.Repeat("x", 65)}, "at most 64"},
		{map[string]string{"DENAR_BOT_RATE": "5"}, "DENAR_BOT_RATE must be"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			clean(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := FromEnv()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "a long admin password") {
				t.Error("error leaks the password")
			}
		})
	}
}

func TestAdminPasswordFile(t *testing.T) {
	clean(t)
	path := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(path, []byte("a password from a file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DENAR_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("DENAR_ADMIN_PASSWORD_FILE", path)
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminPassword.Reveal() != "a password from a file" {
		t.Errorf("password = %q", cfg.AdminPassword.Reveal())
	}
	t.Setenv("DENAR_ADMIN_PASSWORD", "another long password")
	if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), "set only one") {
		t.Errorf("both set: err = %v", err)
	}
}

func TestSecretIsRedacted(t *testing.T) {
	s := Secret("hunter2-hunter2")
	var b strings.Builder
	log := slog.New(slog.NewJSONHandler(&b, nil))
	log.Info("x", "cfg", Config{AdminPassword: s, OIDCClientSecret: s})
	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%#v", s), b.String()} {
		if strings.Contains(out, "hunter2") {
			t.Errorf("secret leaked: %s", out)
		}
	}
	if s.Reveal() != "hunter2-hunter2" || Secret("").String() != "" {
		t.Error("Reveal or empty String wrong")
	}
}
