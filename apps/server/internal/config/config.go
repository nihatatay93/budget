package config

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment  string
	HTTPAddr     string
	PublicOrigin string
	DatabaseURL  string
	SessionTTL   time.Duration
	CookieSecure bool
	// ExchangeRates is off unless an operator opts in, so a self-hosted deployment makes no
	// outbound requests by default.
	ExchangeRates ExchangeRatesConfig
	// SMTP is off unless an operator opts in. Without it invitations still work: the
	// acceptance token is disclosed once to the inviter to share directly.
	SMTP SMTPConfig
	// TrustedProxies are networks whose X-Forwarded-For may be believed when identifying a
	// client. Empty means the immediate peer is used.
	TrustedProxies []*net.IPNet
	// AuthRateLimit throttles credential endpoints per client per minute.
	AuthRateLimit int
	AuthRateBurst int
}

// SMTPConfig controls optional outbound invitation email.
type SMTPConfig struct {
	Enabled     bool
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

// ExchangeRatesConfig controls the optional display-conversion rate feed.
type ExchangeRatesConfig struct {
	Enabled bool
	BaseURL string
	Timeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:  valueOrDefault("BUDGET_ENV", "development"),
		HTTPAddr:     valueOrDefault("BUDGET_HTTP_ADDR", ":8080"),
		PublicOrigin: valueOrDefault("BUDGET_PUBLIC_ORIGIN", "http://localhost:5173"),
		DatabaseURL:  os.Getenv("BUDGET_DATABASE_URL"),
		SessionTTL:   30 * 24 * time.Hour,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("BUDGET_DATABASE_URL is required")
	}
	origin, err := url.Parse(cfg.PublicOrigin)
	if err != nil || origin.Scheme == "" || origin.Host == "" || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return Config{}, errors.New("BUDGET_PUBLIC_ORIGIN must be an origin without a path")
	}
	secureDefault := "true"
	if isLoopbackHost(origin.Hostname()) {
		secureDefault = "false"
	}
	cookieSecure, err := strconv.ParseBool(valueOrDefault("BUDGET_SESSION_COOKIE_SECURE", secureDefault))
	if err != nil {
		return Config{}, fmt.Errorf("parse BUDGET_SESSION_COOKIE_SECURE: %w", err)
	}
	cfg.CookieSecure = cookieSecure
	if value := os.Getenv("BUDGET_SESSION_TTL"); value != "" {
		ttl, err := time.ParseDuration(value)
		if err != nil || ttl <= 0 {
			return Config{}, errors.New("BUDGET_SESSION_TTL must be a positive Go duration")
		}
		cfg.SessionTTL = ttl
	}
	exchangeRates, err := loadExchangeRates()
	if err != nil {
		return Config{}, err
	}
	cfg.ExchangeRates = exchangeRates
	smtp, err := loadSMTP()
	if err != nil {
		return Config{}, err
	}
	cfg.SMTP = smtp
	trusted, err := loadTrustedProxies()
	if err != nil {
		return Config{}, err
	}
	cfg.TrustedProxies = trusted
	cfg.AuthRateLimit, err = positiveInt("BUDGET_AUTH_RATE_LIMIT", 20)
	if err != nil {
		return Config{}, err
	}
	cfg.AuthRateBurst, err = positiveInt("BUDGET_AUTH_RATE_BURST", 10)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// loadTrustedProxies parses the networks whose forwarded-for headers are believed.
//
// Empty by default: believing the header from an untrusted peer would let any caller reset
// their own rate limit by inventing an address.
func loadTrustedProxies() ([]*net.IPNet, error) {
	value := strings.TrimSpace(os.Getenv("BUDGET_TRUSTED_PROXIES"))
	if value == "" {
		return nil, nil
	}
	var networks []*net.IPNet
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// A bare address is accepted as a single-host network for convenience.
		if !strings.Contains(entry, "/") {
			if address := net.ParseIP(entry); address != nil {
				bits := 32
				if address.To4() == nil {
					bits = 128
				}
				entry = fmt.Sprintf("%s/%d", entry, bits)
			}
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("BUDGET_TRUSTED_PROXIES entry %q is not an address or CIDR", entry)
		}
		networks = append(networks, network)
	}
	return networks, nil
}

// positiveInt reads a non-negative integer setting, where zero disables the feature.
func positiveInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

func loadSMTP() (SMTPConfig, error) {
	enabled, err := strconv.ParseBool(valueOrDefault("BUDGET_SMTP_ENABLED", "false"))
	if err != nil {
		return SMTPConfig{}, fmt.Errorf("parse BUDGET_SMTP_ENABLED: %w", err)
	}
	cfg := SMTPConfig{
		Enabled:     enabled,
		Host:        os.Getenv("BUDGET_SMTP_HOST"),
		Port:        587,
		Username:    os.Getenv("BUDGET_SMTP_USERNAME"),
		Password:    os.Getenv("BUDGET_SMTP_PASSWORD"),
		FromAddress: os.Getenv("BUDGET_SMTP_FROM_ADDRESS"),
		FromName:    valueOrDefault("BUDGET_SMTP_FROM_NAME", "Budget"),
		Timeout:     15 * time.Second,
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	// Validate only when the feature is on, so a deployment that never enables it cannot fail
	// startup over a value it does not use.
	if value := os.Getenv("BUDGET_SMTP_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 || port > 65535 {
			return SMTPConfig{}, errors.New("BUDGET_SMTP_PORT must be a TCP port number")
		}
		cfg.Port = port
	}
	if value := os.Getenv("BUDGET_SMTP_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return SMTPConfig{}, errors.New("BUDGET_SMTP_TIMEOUT must be a positive Go duration")
		}
		cfg.Timeout = timeout
	}
	if cfg.Host == "" {
		return SMTPConfig{}, errors.New("BUDGET_SMTP_HOST is required when SMTP is enabled")
	}
	// Providers authenticate the sender, so credentials are required rather than optional.
	if cfg.Username == "" || cfg.Password == "" {
		return SMTPConfig{}, errors.New(
			"BUDGET_SMTP_USERNAME and BUDGET_SMTP_PASSWORD are required when SMTP is enabled",
		)
	}
	if cfg.FromAddress == "" {
		// Most providers require the envelope sender to match the authenticated account.
		cfg.FromAddress = cfg.Username
	}
	if _, err := mail.ParseAddress(cfg.FromAddress); err != nil {
		return SMTPConfig{}, errors.New("BUDGET_SMTP_FROM_ADDRESS must be a valid email address")
	}
	return cfg, nil
}

func loadExchangeRates() (ExchangeRatesConfig, error) {
	enabled, err := strconv.ParseBool(valueOrDefault("BUDGET_EXCHANGE_RATES_ENABLED", "false"))
	if err != nil {
		return ExchangeRatesConfig{}, fmt.Errorf("parse BUDGET_EXCHANGE_RATES_ENABLED: %w", err)
	}
	cfg := ExchangeRatesConfig{
		Enabled: enabled,
		BaseURL: valueOrDefault("BUDGET_EXCHANGE_RATES_BASE_URL", "https://api.frankfurter.dev"),
		Timeout: 10 * time.Second,
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	// Validate the remaining settings only when the feature is on, so a deployment that never
	// enables it cannot fail startup over a value it does not use.
	endpoint, err := url.Parse(cfg.BaseURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil ||
		endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return ExchangeRatesConfig{}, errors.New("BUDGET_EXCHANGE_RATES_BASE_URL must be an https origin")
	}
	if value := os.Getenv("BUDGET_EXCHANGE_RATES_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return ExchangeRatesConfig{}, errors.New("BUDGET_EXCHANGE_RATES_TIMEOUT must be a positive Go duration")
		}
		cfg.Timeout = timeout
	}
	return cfg, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
