package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nihatatay93/budget/internal/account"
	httpapi "github.com/nihatatay93/budget/internal/api/http"
	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/config"
	"github.com/nihatatay93/budget/internal/exchange"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
	frankfurterplatform "github.com/nihatatay93/budget/internal/platform/frankfurter"
	mailplatform "github.com/nihatatay93/budget/internal/platform/mail"
	postgresplatform "github.com/nihatatay93/budget/internal/platform/postgres"
	"github.com/nihatatay93/budget/internal/reporting"
	"github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

// systemClock is the real time source for exchange-rate freshness checks.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type exchangeCapabilities struct {
	display *exchange.WorkspaceService
	booking transaction.BookingRateResolver
}

// invitationNotifier returns nil when the operator has not configured SMTP.
func invitationNotifier(cfg config.Config, logger *slog.Logger) *mailplatform.InvitationNotifier {
	if !cfg.SMTP.Enabled {
		logger.Info("SMTP disabled; invitation codes are shared by the inviter")
		return nil
	}
	sender := mailplatform.NewSender(mailplatform.Options{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		Timeout:  cfg.SMTP.Timeout,
	})
	return mailplatform.NewInvitationNotifier(
		sender, cfg.SMTP.FromAddress, cfg.SMTP.FromName, cfg.PublicOrigin,
	)
}

// exchangeServices returns empty capabilities when the operator has not opted in. Foreign
// transactions can still be booked with an explicit base amount in that configuration.
func exchangeServices(
	cfg config.Config,
	store *postgresplatform.Store,
	access *workspace.Authorizer,
	workspaces exchange.WorkspaceRepository,
	logger *slog.Logger,
) exchangeCapabilities {
	if !cfg.ExchangeRates.Enabled {
		logger.Info("exchange rates disabled; currency conversion will not be offered")
		return exchangeCapabilities{}
	}
	provider := frankfurterplatform.New(cfg.ExchangeRates.BaseURL, cfg.ExchangeRates.Timeout)
	repository := postgresplatform.NewExchangeRateRepository(store.Pool())
	rates := exchange.NewService(
		repository, provider, systemClock{}, logger,
	)
	return exchangeCapabilities{
		display: exchange.NewWorkspaceService(rates, access, workspaces),
		booking: exchange.NewBookingService(provider, repository, systemClock{}),
	}
}

type App struct {
	handler http.Handler
	store   *postgresplatform.Store
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := postgresplatform.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	store, err := postgresplatform.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	authentication, err := auth.NewService(
		postgresplatform.NewAuthRepository(store.Pool()),
		cryptoplatform.PasswordHasher{},
		cfg.SessionTTL,
	)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("create authentication service: %w", err)
	}
	workspaces := postgresplatform.NewWorkspaceRepository(store.Pool())
	access := workspace.NewAuthorizer(workspaces)
	accounts := account.NewService(postgresplatform.NewAccountRepository(store.Pool()), access)
	categories := category.NewService(postgresplatform.NewCategoryRepository(store.Pool()), access)
	exchangeServices := exchangeServices(cfg, store, access, workspaces, logger)
	transactions := transaction.NewService(
		postgresplatform.NewTransactionRepository(store.Pool()), access, exchangeServices.booking,
	)
	reports := reporting.NewService(
		postgresplatform.NewReportingRepository(store.Pool()), access, time.Now,
	)
	budgets := budget.NewService(
		postgresplatform.NewBudgetRepository(store.Pool()), access, time.Now,
	)
	collaboration := workspace.NewCollaborationService(
		postgresplatform.NewCollaborationRepository(store.Pool()), access, time.Now,
	)
	// Left unset when the operator has not configured SMTP, in which case the acceptance
	// token is disclosed to the inviter to share directly.
	if notifier := invitationNotifier(cfg, logger); notifier != nil {
		collaboration = collaboration.WithInvitationNotifier(notifier, workspaces, logger)
	}
	services := httpapi.Services{
		Database:       store.Pool(),
		Authentication: authentication,
		Accounts:       accounts,
		Categories:     categories,
		Transactions:   transactions,
		Budgets:        budgets,
		Reporting:      reports,
		Collaboration:  collaboration,
	}
	// Left nil when the operator has not enabled rate fetching, which the rates endpoint
	// reports as unavailable.
	if exchangeServices.display != nil {
		services.ExchangeRates = exchangeServices.display
	}
	handler, err := httpapi.NewRouter(services, httpapi.Options{
		PublicOrigin:   cfg.PublicOrigin,
		CookieSecure:   cfg.CookieSecure,
		Logger:         logger,
		TrustedProxies: cfg.TrustedProxies,
		AuthRateLimit:  cfg.AuthRateLimit,
		AuthRateBurst:  cfg.AuthRateBurst,
		// The cookie is marked Secure exactly when the deployment terminates TLS, which is
		// also when pinning the browser to HTTPS is correct.
		HSTS: cfg.CookieSecure,
	})
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("create HTTP router: %w", err)
	}

	return &App{handler: handler, store: store}, nil
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) Close() {
	a.store.Close()
}
