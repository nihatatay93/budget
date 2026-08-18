package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("BUDGET_DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing database URL error")
	}
}

func TestLoadAuthenticationDefaults(t *testing.T) {
	t.Setenv("BUDGET_DATABASE_URL", "postgres://budget:budget@localhost/budget")
	t.Setenv("BUDGET_PUBLIC_ORIGIN", "http://localhost:5173")
	t.Setenv("BUDGET_SESSION_COOKIE_SECURE", "")
	t.Setenv("BUDGET_SESSION_TTL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CookieSecure {
		t.Fatal("CookieSecure = true for local HTTP development")
	}
	if cfg.SessionTTL != 30*24*time.Hour {
		t.Fatalf("SessionTTL = %v, want 30 days", cfg.SessionTTL)
	}
}

func TestLoadRejectsInvalidSessionTTL(t *testing.T) {
	t.Setenv("BUDGET_DATABASE_URL", "postgres://budget:budget@localhost/budget")
	t.Setenv("BUDGET_SESSION_TTL", "never")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid session TTL error")
	}
}

func TestLoadExchangeRates(t *testing.T) {
	t.Setenv("BUDGET_DATABASE_URL", "postgres://budget:budget@localhost/budget")
	t.Setenv("BUDGET_EXCHANGE_RATES_ENABLED", "true")
	t.Setenv("BUDGET_EXCHANGE_RATES_BASE_URL", "https://rates.example")
	t.Setenv("BUDGET_EXCHANGE_RATES_TIMEOUT", "3s")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.ExchangeRates.Enabled || cfg.ExchangeRates.Timeout != 3*time.Second {
		t.Fatalf("ExchangeRates = %+v", cfg.ExchangeRates)
	}
}

func TestLoadRejectsExchangeRateURLThatIsNotHTTPSOrigin(t *testing.T) {
	for _, value := range []string{
		"http://rates.example",
		"https://rates.example/proxy",
		"https://rates.example?token=secret",
		"https://user:pass@rates.example",
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("BUDGET_DATABASE_URL", "postgres://budget:budget@localhost/budget")
			t.Setenv("BUDGET_EXCHANGE_RATES_ENABLED", "true")
			t.Setenv("BUDGET_EXCHANGE_RATES_BASE_URL", value)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want invalid exchange-rate origin error")
			}
		})
	}
}
