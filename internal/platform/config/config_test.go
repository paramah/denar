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
	"DENAR_NBP_URL", "DENAR_FX_INTERVAL", "DENAR_CHARGE_HORIZON", "DENAR_SUBSCRIPTION_INTERVAL",
	"DENAR_MASTER_KEY", "DENAR_MASTER_KEY_FILE", "DENAR_SMTP_HOST", "DENAR_SMTP_PORT", "DENAR_SMTP_USERNAME",
	"DENAR_SMTP_PASSWORD", "DENAR_SMTP_PASSWORD_FILE", "DENAR_SMTP_FROM", "DENAR_SMTP_TLS", "DENAR_NOTIFY_INTERVAL",
	"DENAR_BUDGET_CHECK_INTERVAL", "DENAR_GOAL_DUE_DAYS", "DENAR_TELEGRAM_API_URL", "DENAR_DEV_FAKE_CONNECTOR",
	"DENAR_NOTIFY_ALLOW_PRIVATE", "DENAR_NOTIFY_ALLOWED_CIDRS",
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
	if cfg.Addr != ":8080" || cfg.Database != "denar.db" || !cfg.LocalLogin || cfg.AutoMigrate ||
		cfg.SessionTTL != 168*time.Hour || cfg.InviteTTL != 168*time.Hour || cfg.BaseURL != "http://localhost:8080" ||
		cfg.InstanceName != "Denar" || cfg.BotRate != 120 || cfg.OIDCAutoCreate || cfg.HasAdmin() || cfg.OIDCEnabled() ||
		cfg.NBPURL != "https://api.nbp.pl" || cfg.FxInterval != 30*time.Minute || cfg.ChargeHorizonDays != 90 ||
		cfg.SubscriptionInterval != time.Hour || cfg.NotifyAllowPrivate || len(cfg.NotifyAllowedCIDRs) != 0 {
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
	t.Setenv("DENAR_NOTIFY_ALLOW_PRIVATE", "true")
	t.Setenv("DENAR_NOTIFY_ALLOWED_CIDRS", "192.168.1.0/24, 10.0.0.7, fd00::/8")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HasAdmin() || !cfg.OIDCEnabled() || !cfg.OIDCAutoCreate || cfg.BaseURL != "https://denar.example.com" ||
		len(cfg.TrustedProxies) != 2 || cfg.InviteTTL != 48*time.Hour || cfg.BotRate != 60 ||
		!cfg.NotifyAllowPrivate || len(cfg.NotifyAllowedCIDRs) != 3 || cfg.NotifyAllowedCIDRs[1].String() != "10.0.0.7/32" {
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
		{map[string]string{"DENAR_NOTIFY_ALLOW_PRIVATE": "yes please"}, "DENAR_NOTIFY_ALLOW_PRIVATE must be true or false"},
		{map[string]string{"DENAR_NOTIFY_ALLOWED_CIDRS": "lan"}, "DENAR_NOTIFY_ALLOWED_CIDRS must list networks such as 192.168.1.0/24"},
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
	log.Info("x", "cfg", Config{AdminPassword: s, OIDCClientSecret: s, Database: "postgres://denar:hunter2-hunter2@db:5432/denar"})
	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%#v", s), b.String()} {
		if strings.Contains(out, "hunter2") {
			t.Errorf("secret leaked: %s", out)
		}
	}
	if s.Reveal() != "hunter2-hunter2" || Secret("").String() != "" {
		t.Error("Reveal or empty String wrong")
	}
}

func TestNBP(t *testing.T) {
	clean(t)
	t.Setenv("DENAR_NBP_URL", "http://127.0.0.1:9999/")
	t.Setenv("DENAR_FX_INTERVAL", "5m")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NBPURL != "http://127.0.0.1:9999" || cfg.FxInterval != 5*time.Minute {
		t.Errorf("cfg = %+v", cfg)
	}
	for key, bad := range map[string]string{"DENAR_NBP_URL": "ftp://x", "DENAR_FX_INTERVAL": "-1m"} {
		clean(t)
		t.Setenv(key, bad)
		if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%s: err = %v", key, bad, err)
		}
	}
}

func TestSubscriptions(t *testing.T) {
	clean(t)
	t.Setenv("DENAR_CHARGE_HORIZON", "120")
	t.Setenv("DENAR_SUBSCRIPTION_INTERVAL", "15m")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChargeHorizonDays != 120 || cfg.SubscriptionInterval != 15*time.Minute {
		t.Errorf("cfg = %+v", cfg)
	}
	for key, bad := range map[string]string{"DENAR_CHARGE_HORIZON": "6", "DENAR_SUBSCRIPTION_INTERVAL": "soon"} {
		clean(t)
		t.Setenv(key, bad)
		if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%s: err = %v", key, bad, err)
		}
	}
	clean(t)
	t.Setenv("DENAR_CHARGE_HORIZON", "731")
	if _, err := FromEnv(); err == nil {
		t.Error("horizon of 731 days")
	}
}

func TestNotificationSettings(t *testing.T) {
	clean(t)
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MasterKey != "" || cfg.SMTPEnabled() || cfg.NotifyInterval != 30*time.Second || cfg.BudgetCheckInterval != 15*time.Minute ||
		cfg.GoalDueDays != 7 || cfg.TelegramAPIURL != "https://api.telegram.org" || cfg.DevFakeConnector {
		t.Errorf("defaults = %+v", cfg)
	}

	key := "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA="
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DENAR_MASTER_KEY_FILE", path)
	t.Setenv("DENAR_SMTP_HOST", "smtp.example.com")
	t.Setenv("DENAR_SMTP_FROM", "Denar <denar@example.com>")
	t.Setenv("DENAR_SMTP_USERNAME", "denar")
	t.Setenv("DENAR_SMTP_PASSWORD", "smtp-s3cret")
	t.Setenv("DENAR_SMTP_TLS", "tls")
	t.Setenv("DENAR_GOAL_DUE_DAYS", "14")
	t.Setenv("DENAR_TELEGRAM_API_URL", "http://127.0.0.1:9000/")
	t.Setenv("DENAR_DEV_FAKE_CONNECTOR", "1")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MasterKey.Reveal() != key || !cfg.SMTPEnabled() || cfg.SMTPPort != 465 || cfg.SMTPTLS != "tls" || cfg.SMTPPassword.Reveal() != "smtp-s3cret" ||
		cfg.GoalDueDays != 14 || cfg.TelegramAPIURL != "http://127.0.0.1:9000" || !cfg.DevFakeConnector {
		t.Errorf("values = %+v", cfg)
	}
	logged := fmt.Sprint(slog.GroupValue(cfg.LogValue().Group()...))
	if strings.Contains(logged, key) || strings.Contains(logged, "smtp-s3cret") {
		t.Errorf("a secret is logged: %s", logged)
	}

	clean(t)
	t.Setenv("DENAR_MASTER_KEY", "c2hvcnQ=")
	t.Setenv("DENAR_SMTP_HOST", "smtp.example.com")
	t.Setenv("DENAR_SMTP_TLS", "ssl")
	t.Setenv("DENAR_SMTP_USERNAME", "denar")
	t.Setenv("DENAR_GOAL_DUE_DAYS", "0")
	_, err = FromEnv()
	for _, want := range []string{"DENAR_MASTER_KEY", "DENAR_SMTP_TLS", "DENAR_SMTP_FROM", "DENAR_SMTP_PASSWORD", "DENAR_GOAL_DUE_DAYS"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not name %s", err, want)
		}
	}
}
