package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/nihatatay93/budget/internal/exchange"
)

// testServices supplies a working stub for every required collaborator so each test only
// names the service it actually exercises.
func testServices() Services {
	return Services{
		Database:       healthyDatabase{},
		Authentication: &fakeAuthService{principal: testAuthResult("bearer").Principal},
		Accounts:       &fakeAccountService{},
		Categories:     &fakeCategoryService{},
		Transactions:   &fakeTransactionService{},
		Budgets:        &fakeBudgetService{},
		Reporting:      &fakeReportingService{},
		Collaboration:  &fakeCollaborationService{},
	}
}

func testOptions() Options {
	return Options{
		PublicOrigin: "https://budget.example",
		CookieSecure: true,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func testRouter(t *testing.T, services Services) http.Handler {
	t.Helper()
	router, err := NewRouter(services, testOptions())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return router
}

// disabledExchangeRates mirrors how app.go supplies the optional service when the operator
// has not enabled rate fetching: a nil *exchange.WorkspaceService, not a nil interface.
// An interface holding that pointer is not equal to nil, so a plain guard would let the
// request through and panic on first use.
func disabledExchangeRates() exchangeRateService {
	var disabled *exchange.WorkspaceService
	return disabled
}

func TestNewRouterReportsEveryMissingRequiredService(t *testing.T) {
	_, err := NewRouter(Services{Database: healthyDatabase{}}, testOptions())
	if err == nil {
		t.Fatal("NewRouter() error = nil, want a startup failure listing missing services")
	}
	for _, name := range []string{
		"Authentication", "Accounts", "Categories", "Transactions",
		"Budgets", "Reporting", "Collaboration",
	} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("NewRouter() error = %q, missing %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "ExchangeRates") {
		t.Fatalf("NewRouter() error = %q, ExchangeRates is optional", err)
	}
}

// A required service supplied as a typed nil must fail at startup rather than at request
// time, which a plain == nil check cannot detect.
func TestNewRouterRejectsTypedNilRequiredService(t *testing.T) {
	services := testServices()
	var missing *fakeAccountService
	services.Accounts = missing

	if _, err := NewRouter(services, testOptions()); err == nil {
		t.Fatal("NewRouter() accepted a typed-nil required service")
	}
}

func TestNewRouterRequiresLogger(t *testing.T) {
	options := testOptions()
	options.Logger = nil
	if _, err := NewRouter(testServices(), options); err == nil {
		t.Fatal("NewRouter() accepted nil logger")
	}
}
