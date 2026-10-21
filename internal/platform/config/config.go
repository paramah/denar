// Package config loads runtime configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"git.eiac.dev/tools/denar/internal/platform/crypto"
)

// minAdminPasswordLen is the shortest bootstrap admin password accepted.
const minAdminPasswordLen = 12

// maxInstanceNameLen is the longest instance name accepted, in characters.
const maxInstanceNameLen = 64

// Limits of the bot request rate.
const (
	minBotRate = 10
	maxBotRate = 6000
)

// Config is the server's runtime configuration.
type Config struct {
	Addr         string // DENAR_ADDR, listen address
	DatabasePath string // DENAR_DB, SQLite file

	// Bootstrap admin account; email and password are set together or not
	// at all.
	AdminEmail    string // DENAR_ADMIN_EMAIL
	AdminPassword Secret // DENAR_ADMIN_PASSWORD or DENAR_ADMIN_PASSWORD_FILE
	AdminName     string // DENAR_ADMIN_NAME, display name

	LocalLogin bool          // DENAR_LOCAL_LOGIN, password login enabled
	BaseURL    string        // DENAR_BASE_URL, public origin without a trailing slash
	SessionTTL time.Duration // DENAR_SESSION_TTL, idle time after which a session ends
	InviteTTL  time.Duration // DENAR_INVITE_TTL, how long an invitation link works

	OIDCIssuer       string // DENAR_OIDC_ISSUER
	OIDCClientID     string // DENAR_OIDC_CLIENT_ID
	OIDCClientSecret Secret // DENAR_OIDC_CLIENT_SECRET
	// OIDCAutoCreate lets any user of the identity provider get an account
	// on first login (DENAR_OIDC_AUTO_CREATE). Off, only invited emails do.
	OIDCAutoCreate bool

	AutoMigrate bool // DENAR_AUTO_MIGRATE, migrate the schema when serve starts

	// TrustedProxies are the networks of the reverse proxies in front of
	// the server (DENAR_TRUSTED_PROXIES, comma-separated CIDRs or
	// addresses). Only they are believed about the client's address.
	TrustedProxies []netip.Prefix

	InstanceName string // DENAR_INSTANCE_NAME, shown on the login screen

	// BotRate is how many requests a bot may make per minute
	// (DENAR_BOT_RATE, 10 to 6000).
	BotRate int

	// NBPURL is the origin of the NBP rates API (DENAR_NBP_URL).
	NBPURL string
	// FxInterval is how often missing base amounts are filled in
	// (DENAR_FX_INTERVAL).
	FxInterval time.Duration

	// ChargeHorizonDays is how many days ahead subscription charges are
	// expected (DENAR_CHARGE_HORIZON, 7 to 730).
	ChargeHorizonDays int
	// SubscriptionInterval is how often the subscription jobs run
	// (DENAR_SUBSCRIPTION_INTERVAL).
	SubscriptionInterval time.Duration

	// MasterKey encrypts channel secrets (DENAR_MASTER_KEY or
	// DENAR_MASTER_KEY_FILE): 32 bytes, base64. Unset, channels with
	// secrets are unavailable.
	MasterKey Secret
	// SMTP sends e-mail; SMTPHost "" means no e-mail (DENAR_SMTP_*).
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword Secret
	SMTPFrom     string
	SMTPTLS      string // starttls, tls or none
	// NotifyInterval is how often events are dispatched and deliveries
	// sent (DENAR_NOTIFY_INTERVAL).
	NotifyInterval time.Duration
	// BudgetCheckInterval is how often envelopes are checked for the
	// budget events (DENAR_BUDGET_CHECK_INTERVAL).
	BudgetCheckInterval time.Duration
	// GoalDueDays is how many days ahead a goal is reminded of
	// (DENAR_GOAL_DUE_DAYS, 1 to 60).
	GoalDueDays int
	// TelegramAPIURL is the Bot API origin (DENAR_TELEGRAM_API_URL).
	TelegramAPIURL string
}

// SMTPEnabled reports whether e-mail can be sent.
func (c Config) SMTPEnabled() bool { return c.SMTPHost != "" }

// FromEnv reads and checks the whole configuration, falling back to
// defaults for unset variables. The error lists every problem found and
// never contains a secret.
func FromEnv() (Config, error) {
	var errs []error
	cfg := Config{
		Addr:         env("DENAR_ADDR", ":8080"),
		DatabasePath: env("DENAR_DB", "denar.db"),
	}
	cfg.AdminEmail = os.Getenv("DENAR_ADMIN_EMAIL")
	cfg.AdminName = env("DENAR_ADMIN_NAME", "Admin")
	cfg.OIDCIssuer = os.Getenv("DENAR_OIDC_ISSUER")
	cfg.OIDCClientID = os.Getenv("DENAR_OIDC_CLIENT_ID")
	cfg.OIDCClientSecret = Secret(os.Getenv("DENAR_OIDC_CLIENT_SECRET"))

	var err error
	if cfg.LocalLogin, err = envBool("DENAR_LOCAL_LOGIN", true); err != nil {
		errs = append(errs, err)
	}
	if cfg.AutoMigrate, err = envBool("DENAR_AUTO_MIGRATE", false); err != nil {
		errs = append(errs, err)
	}
	if cfg.OIDCAutoCreate, err = envBool("DENAR_OIDC_AUTO_CREATE", false); err != nil {
		errs = append(errs, err)
	}
	if cfg.SessionTTL, err = duration("DENAR_SESSION_TTL", "168h"); err != nil {
		errs = append(errs, err)
	}
	if cfg.InviteTTL, err = duration("DENAR_INVITE_TTL", "168h"); err != nil {
		errs = append(errs, err)
	}
	if cfg.AdminPassword, err = adminPassword(); err != nil {
		errs = append(errs, err)
	} else if err := checkAdmin(cfg); err != nil {
		errs = append(errs, err)
	}
	if cfg.BaseURL, err = baseURL(cfg.OIDCEnabled()); err != nil {
		errs = append(errs, err)
	}
	if cfg.TrustedProxies, err = trustedProxies(); err != nil {
		errs = append(errs, err)
	}
	if cfg.InstanceName, err = instanceName(); err != nil {
		errs = append(errs, err)
	}
	if cfg.BotRate, err = botRate(); err != nil {
		errs = append(errs, err)
	}
	if cfg.NBPURL, err = nbpURL(); err != nil {
		errs = append(errs, err)
	}
	if cfg.FxInterval, err = duration("DENAR_FX_INTERVAL", "30m"); err != nil {
		errs = append(errs, err)
	}
	if cfg.ChargeHorizonDays, err = chargeHorizon(); err != nil {
		errs = append(errs, err)
	}
	if cfg.SubscriptionInterval, err = duration("DENAR_SUBSCRIPTION_INTERVAL", "1h"); err != nil {
		errs = append(errs, err)
	}
	if cfg.MasterKey, err = secretFromEnv("DENAR_MASTER_KEY"); err != nil {
		errs = append(errs, err)
	} else if cfg.MasterKey != "" {
		if _, err := crypto.ParseKey(cfg.MasterKey.Reveal()); err != nil {
			errs = append(errs, fmt.Errorf("DENAR_MASTER_KEY: %v (generate one with: openssl rand -base64 32)", err))
		}
	}
	if err := smtpConfig(&cfg); err != nil {
		errs = append(errs, err)
	}
	if cfg.NotifyInterval, err = duration("DENAR_NOTIFY_INTERVAL", "30s"); err != nil {
		errs = append(errs, err)
	}
	if cfg.BudgetCheckInterval, err = duration("DENAR_BUDGET_CHECK_INTERVAL", "15m"); err != nil {
		errs = append(errs, err)
	}
	if cfg.GoalDueDays, err = intBetween("DENAR_GOAL_DUE_DAYS", "7", 1, 60); err != nil {
		errs = append(errs, err)
	}
	if cfg.TelegramAPIURL, err = originURL("DENAR_TELEGRAM_API_URL", "https://api.telegram.org"); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// HasAdmin reports whether a bootstrap admin account is configured.
func (c Config) HasAdmin() bool {
	return c.AdminEmail != "" && c.AdminPassword != ""
}

// OIDCEnabled reports whether OIDC login is configured, which takes the
// issuer, the client ID and the client secret.
func (c Config) OIDCEnabled() bool {
	return c.OIDCIssuer != "" && c.OIDCClientID != "" && c.OIDCClientSecret != ""
}

// LogValue lets the configuration be logged; Secret fields redact
// themselves.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("addr", c.Addr),
		slog.String("database_path", c.DatabasePath),
		slog.String("admin_email", c.AdminEmail),
		slog.Any("admin_password", c.AdminPassword),
		slog.String("admin_name", c.AdminName),
		slog.Bool("local_login", c.LocalLogin),
		slog.String("base_url", c.BaseURL),
		slog.Duration("session_ttl", c.SessionTTL),
		slog.Duration("invite_ttl", c.InviteTTL),
		slog.String("oidc_issuer", c.OIDCIssuer),
		slog.String("oidc_client_id", c.OIDCClientID),
		slog.Any("oidc_client_secret", c.OIDCClientSecret),
		slog.Bool("oidc_auto_create", c.OIDCAutoCreate),
		slog.Bool("auto_migrate", c.AutoMigrate),
		slog.Any("trusted_proxies", c.TrustedProxies),
		slog.String("instance_name", c.InstanceName),
		slog.Int("bot_rate", c.BotRate),
		slog.String("nbp_url", c.NBPURL),
		slog.Duration("fx_interval", c.FxInterval),
		slog.Int("charge_horizon_days", c.ChargeHorizonDays),
		slog.Duration("subscription_interval", c.SubscriptionInterval),
		slog.Any("master_key", c.MasterKey),
		slog.String("smtp_host", c.SMTPHost),
		slog.Int("smtp_port", c.SMTPPort),
		slog.String("smtp_username", c.SMTPUsername),
		slog.Any("smtp_password", c.SMTPPassword),
		slog.String("smtp_from", c.SMTPFrom),
		slog.String("smtp_tls", c.SMTPTLS),
		slog.Duration("notify_interval", c.NotifyInterval),
		slog.Duration("budget_check_interval", c.BudgetCheckInterval),
		slog.Int("goal_due_days", c.GoalDueDays),
		slog.String("telegram_api_url", c.TelegramAPIURL),
	)
}

// Secret is a configuration value that must not be printed. Formatting,
// logging and encoding it give a placeholder; Reveal returns the value.
type Secret string

const redacted = "[redacted]"

// Reveal returns the secret itself.
func (s Secret) Reveal() string { return string(s) }

// String returns a placeholder, or "" when the secret is unset.
func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return redacted
}

func (s Secret) GoString() string { return strconv.Quote(s.String()) }

func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }

func (s Secret) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

func duration(key, fallback string) (time.Duration, error) {
	d, err := time.ParseDuration(env(key, fallback))
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as %s", key, fallback)
	}
	return d, nil
}

// Limits of the charge horizon, in days.
const (
	minChargeHorizon = 7
	maxChargeHorizon = 730
)

func chargeHorizon() (int, error) {
	const key = "DENAR_CHARGE_HORIZON"
	days, err := strconv.Atoi(env(key, "90"))
	if err != nil || days < minChargeHorizon || days > maxChargeHorizon {
		return 0, fmt.Errorf("%s must be a number of days between %d and %d", key, minChargeHorizon, maxChargeHorizon)
	}
	return days, nil
}

func botRate() (int, error) {
	const key = "DENAR_BOT_RATE"
	rate, err := strconv.Atoi(env(key, "120"))
	if err != nil || rate < minBotRate || rate > maxBotRate {
		return 0, fmt.Errorf("%s must be a number of requests per minute between %d and %d", key, minBotRate, maxBotRate)
	}
	return rate, nil
}

// adminPassword takes the password from the variable or from the file the
// _FILE variant names, as container secrets are mounted.
func adminPassword() (Secret, error) { return secretFromEnv("DENAR_ADMIN_PASSWORD") }

// secretFromEnv reads the variable key or the file key_FILE names.
func secretFromEnv(key string) (Secret, error) {
	fileKey := key + "_FILE"
	password, path := os.Getenv(key), os.Getenv(fileKey)
	if path == "" {
		return Secret(password), nil
	}
	if password != "" {
		return "", fmt.Errorf("set only one of %s and %s", key, fileKey)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", fileKey, err)
	}
	return Secret(strings.TrimRight(string(content), "\r\n")), nil
}

func checkAdmin(cfg Config) error {
	switch {
	case cfg.AdminEmail == "" && cfg.AdminPassword == "":
		return nil
	case cfg.AdminPassword == "":
		return errors.New("DENAR_ADMIN_EMAIL is set, so DENAR_ADMIN_PASSWORD must be set too")
	case cfg.AdminEmail == "":
		return errors.New("DENAR_ADMIN_PASSWORD is set, so DENAR_ADMIN_EMAIL must be set too")
	case utf8.RuneCountInString(cfg.AdminPassword.Reveal()) < minAdminPasswordLen:
		return fmt.Errorf("DENAR_ADMIN_PASSWORD must have at least %d characters", minAdminPasswordLen)
	}
	return nil
}

// trustedProxies reads a comma-separated list of networks; a bare address
// stands for itself alone.
func trustedProxies() ([]netip.Prefix, error) {
	const key = "DENAR_TRUSTED_PROXIES"
	var prefixes []netip.Prefix
	for _, item := range strings.Split(os.Getenv(key), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			addr, addrErr := netip.ParseAddr(item)
			if addrErr != nil {
				return nil, fmt.Errorf("%s must list networks such as 10.0.0.0/8, got %q", key, item)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

// instanceName returns the name the instance shows to people. A name of
// spaces only counts as unset.
func instanceName() (string, error) {
	const key = "DENAR_INSTANCE_NAME"
	name := strings.TrimSpace(os.Getenv(key))
	if name == "" {
		return "Denar", nil
	}
	var errs []error
	if utf8.RuneCountInString(name) > maxInstanceNameLen {
		errs = append(errs, fmt.Errorf("%s must have at most %d characters", key, maxInstanceNameLen))
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		errs = append(errs, fmt.Errorf("%s must not contain control characters", key))
	}
	return name, errors.Join(errs...)
}

// baseURL returns the public origin. It may be left to its default only
// while OIDC is off, because redirect URIs are built from it.
func baseURL(required bool) (string, error) {
	const key = "DENAR_BASE_URL"
	v := os.Getenv(key)
	if v == "" {
		if required {
			return "", fmt.Errorf("%s must be set when OIDC is enabled", key)
		}
		return "http://localhost:8080", nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("%s must be an origin such as https://denar.example.com", key)
	}
	return u.Scheme + "://" + u.Host, nil
}

// nbpURL returns the origin of the NBP API: https://api.nbp.pl unless set.
func nbpURL() (string, error) {
	const key = "DENAR_NBP_URL"
	v := os.Getenv(key)
	if v == "" {
		return "https://api.nbp.pl", nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("%s must be a URL such as https://api.nbp.pl", key)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func intBetween(key, fallback string, lo, hi int) (int, error) {
	n, err := strconv.Atoi(env(key, fallback))
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%s must be a number between %d and %d", key, lo, hi)
	}
	return n, nil
}

// originURL returns an http(s) URL without a trailing slash.
func originURL(key, fallback string) (string, error) {
	v := env(key, fallback)
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("%s must be a URL such as %s", key, fallback)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// smtpConfig reads DENAR_SMTP_*. Without a host e-mail is off and the
// other variables are not checked.
func smtpConfig(cfg *Config) error {
	cfg.SMTPHost = strings.TrimSpace(os.Getenv("DENAR_SMTP_HOST"))
	if cfg.SMTPHost == "" {
		return nil
	}
	var errs []error
	cfg.SMTPUsername = os.Getenv("DENAR_SMTP_USERNAME")
	cfg.SMTPFrom = strings.TrimSpace(os.Getenv("DENAR_SMTP_FROM"))
	var err error
	if cfg.SMTPPassword, err = secretFromEnv("DENAR_SMTP_PASSWORD"); err != nil {
		errs = append(errs, err)
	}
	cfg.SMTPTLS = strings.ToLower(env("DENAR_SMTP_TLS", "starttls"))
	defaultPort := "587"
	switch cfg.SMTPTLS {
	case "starttls":
	case "tls":
		defaultPort = "465"
	case "none":
		defaultPort = "25"
	default:
		errs = append(errs, errors.New("DENAR_SMTP_TLS must be starttls, tls or none"))
	}
	if cfg.SMTPPort, err = intBetween("DENAR_SMTP_PORT", defaultPort, 1, 65535); err != nil {
		errs = append(errs, err)
	}
	if a, err := mail.ParseAddress(cfg.SMTPFrom); err != nil || a.Address == "" {
		errs = append(errs, errors.New("DENAR_SMTP_FROM must be an address such as Denar <denar@example.com> when DENAR_SMTP_HOST is set"))
	}
	if cfg.SMTPUsername != "" && cfg.SMTPPassword == "" {
		errs = append(errs, errors.New("DENAR_SMTP_USERNAME is set, so DENAR_SMTP_PASSWORD must be set too"))
	}
	return errors.Join(errs...)
}
