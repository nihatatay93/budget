//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/config"
)

// journey drives the product through its own HTTP surface, built by the real composition
// root against a real database. Unit tests cover each layer; this covers the wiring between
// them, which is where a working part can still be reachable in the wrong way.
type journey struct {
	t      *testing.T
	server *httptest.Server
	token  string
}

func newJourney(t *testing.T) *journey {
	t.Helper()
	ctx := context.Background()
	container, err := postgrescontainer.Run(
		ctx,
		"postgres:18-alpine",
		postgrescontainer.WithDatabase("budget_journey"),
		postgrescontainer.WithUsername("budget"),
		postgrescontainer.WithPassword("budget"),
		postgrescontainer.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	// Rate fetching stays off, which is the default a self-hoster gets.
	application, err := New(ctx, config.Config{
		Environment:  "test",
		HTTPAddr:     ":0",
		PublicOrigin: "https://budget.example",
		DatabaseURL:  databaseURL,
		SessionTTL:   time.Hour,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build application: %v", err)
	}
	t.Cleanup(application.Close)

	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)
	return &journey{t: t, server: server}
}

// do issues a request as the journey's current session and decodes a successful body.
func (j *journey) do(method, path, body string, out any) int {
	j.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request, err := http.NewRequest(method, j.server.URL+path, reader)
	if err != nil {
		j.t.Fatalf("build request: %v", err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if j.token != "" {
		request.Header.Set("Authorization", "Bearer "+j.token)
	}
	response, err := j.server.Client().Do(request)
	if err != nil {
		j.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()
	payload, _ := io.ReadAll(response.Body)
	if out != nil && response.StatusCode/100 == 2 && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			j.t.Fatalf("decode %s %s: %v (%s)", method, path, err, payload)
		}
	}
	if response.StatusCode/100 != 2 && out == nil {
		j.t.Logf("%s %s -> %d %s", method, path, response.StatusCode, payload)
	}
	return response.StatusCode
}

func (j *journey) mustDo(method, path, body string, out any) {
	j.t.Helper()
	if status := j.do(method, path, body, out); status/100 != 2 {
		j.t.Fatalf("%s %s = %d, want success", method, path, status)
	}
}

type registration struct {
	BearerToken string `json:"bearer_token"`
	User        struct {
		ID string `json:"id"`
	} `json:"user"`
	Workspaces []struct {
		ID           string `json:"id"`
		BaseCurrency string `json:"base_currency"`
		Role         string `json:"role"`
	} `json:"workspaces"`
}

func (j *journey) register(email, workspace string) registration {
	j.t.Helper()
	var result registration
	j.mustDo(http.MethodPost, "/v1/auth/register", fmt.Sprintf(
		`{"email":%q,"password":"a-long-enough-passphrase","display_name":"Person",`+
			`"workspace_name":%q,"base_currency":"TRY","timezone":"Europe/Istanbul",`+
			`"transport":"bearer"}`, email, workspace,
	), &result)
	if result.BearerToken == "" || len(result.Workspaces) != 1 {
		j.t.Fatalf("registration = %#v", result)
	}
	return result
}

// The core money journey: an account, a category, a split expense, a budget covering it, and
// a projection that agrees with what was recorded.
func TestJourneyRecordsSpendingAndReportsItConsistently(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("owner@example.com", "Atay Family")
	journey.token = owner.BearerToken
	workspaceID := owner.Workspaces[0].ID

	var account struct {
		ID           string `json:"id"`
		BalanceMinor int64  `json:"balance_minor"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Checking","type":"bank","currency":"TRY"}`, &account)

	var groceries struct {
		ID string `json:"id"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/categories",
		`{"name":"Groceries","kind":"expense"}`, &groceries)
	var household struct {
		ID string `json:"id"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/categories",
		`{"name":"Household","kind":"expense"}`, &household)

	// One account movement, two category effects: the split from docs/domain-model.md.
	var recorded struct {
		ID string `json:"id"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/transactions", fmt.Sprintf(
		`{"kind":"standard","status":"posted","transaction_date":"2026-08-18","payee":"Migros",
		  "entries":[{"account_id":%q,"amount_minor":-150000}],
		  "allocations":[{"category_id":%q,"amount_base_minor":-100000},
		                 {"category_id":%q,"amount_base_minor":-50000}]}`,
		account.ID, groceries.ID, household.ID,
	), &recorded)

	// The balance is derived from posted entries, never stored, so it must now reflect the
	// transaction without anything having written to the account.
	var accounts struct {
		Accounts []struct {
			ID           string `json:"id"`
			BalanceMinor int64  `json:"balance_minor"`
		} `json:"accounts"`
	}
	journey.mustDo(http.MethodGet, "/v1/workspaces/"+workspaceID+"/accounts", "", &accounts)
	if len(accounts.Accounts) != 1 || accounts.Accounts[0].BalanceMinor != -150000 {
		t.Fatalf("derived balance = %#v, want -150000", accounts.Accounts)
	}

	var budget struct {
		PlannedBaseMinor   int64 `json:"planned_base_minor"`
		UsedBaseMinor      int64 `json:"used_base_minor"`
		RemainingBaseMinor int64 `json:"remaining_base_minor"`
	}
	journey.mustDo(http.MethodPut, "/v1/workspaces/"+workspaceID+"/budgets/2026-08", fmt.Sprintf(
		`{"name":"August","items":[{"category_id":%q,"amount_base_minor":200000}]}`, groceries.ID,
	), &budget)
	// Usage is derived from posted allocations, so the grocery half of the split counts and
	// the household half does not.
	if budget.PlannedBaseMinor != 200000 || budget.UsedBaseMinor != 100000 ||
		budget.RemainingBaseMinor != 100000 {
		t.Fatalf("budget = %#v, want 200000 planned / 100000 used / 100000 remaining", budget)
	}

	var projection struct {
		Summary struct {
			SpendingBaseMinor struct {
				Posted int64 `json:"posted"`
			} `json:"spending_base_minor"`
			BalanceBaseMinor struct {
				Posted int64 `json:"posted"`
			} `json:"balance_base_minor"`
		} `json:"summary"`
	}
	journey.mustDo(http.MethodGet,
		"/v1/workspaces/"+workspaceID+"/financial-projection?from_date=2026-08-01&to_date=2026-08-31",
		"", &projection)
	// ADR 0007 gives category values a reporting orientation: expense allocations are stored
	// negative but reported positive, so spending reads as a magnitude. The balance keeps the
	// ledger's sign, so the two differ deliberately.
	if projection.Summary.SpendingBaseMinor.Posted != 150000 {
		t.Fatalf("projected spending = %d, want 150000", projection.Summary.SpendingBaseMinor.Posted)
	}
	if projection.Summary.BalanceBaseMinor.Posted != -150000 {
		t.Fatalf("projected balance = %d, want -150000", projection.Summary.BalanceBaseMinor.Posted)
	}
}

// A transfer moves value between accounts without becoming income or spending. This is
// invariant 5, and it is the one a naive implementation gets wrong.
func TestJourneyTransferDoesNotCountAsSpending(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("transfer@example.com", "Atay Family")
	journey.token = owner.BearerToken
	workspaceID := owner.Workspaces[0].ID

	var checking, savings struct {
		ID string `json:"id"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Checking","type":"bank","currency":"TRY"}`, &checking)
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Savings","type":"savings","currency":"TRY"}`, &savings)

	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/transactions", fmt.Sprintf(
		`{"kind":"transfer","status":"posted","transaction_date":"2026-08-18",
		  "entries":[{"account_id":%q,"amount_minor":-1000000},
		             {"account_id":%q,"amount_minor":1000000}],
		  "allocations":[]}`, checking.ID, savings.ID,
	), nil)

	var projection struct {
		Summary struct {
			SpendingBaseMinor struct{ Posted int64 } `json:"spending_base_minor"`
			IncomeBaseMinor   struct{ Posted int64 } `json:"income_base_minor"`
			BalanceBaseMinor  struct{ Posted int64 } `json:"balance_base_minor"`
		} `json:"summary"`
	}
	journey.mustDo(http.MethodGet,
		"/v1/workspaces/"+workspaceID+"/financial-projection?from_date=2026-08-01&to_date=2026-08-31",
		"", &projection)
	if projection.Summary.SpendingBaseMinor.Posted != 0 ||
		projection.Summary.IncomeBaseMinor.Posted != 0 {
		t.Fatalf("transfer leaked into reporting: %#v", projection.Summary)
	}
	if projection.Summary.BalanceBaseMinor.Posted != 0 {
		t.Fatalf("net worth changed on a transfer: %d", projection.Summary.BalanceBaseMinor.Posted)
	}
}

// An unbalanced aggregate must be refused by the product, not merely by the database.
func TestJourneyRejectsUnbalancedSplit(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("unbalanced@example.com", "Atay Family")
	journey.token = owner.BearerToken
	workspaceID := owner.Workspaces[0].ID

	var account, category struct {
		ID string `json:"id"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Checking","type":"bank","currency":"TRY"}`, &account)
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/categories",
		`{"name":"Groceries","kind":"expense"}`, &category)

	status := journey.do(http.MethodPost, "/v1/workspaces/"+workspaceID+"/transactions", fmt.Sprintf(
		`{"kind":"standard","status":"posted","transaction_date":"2026-08-18",
		  "entries":[{"account_id":%q,"amount_minor":-150000}],
		  "allocations":[{"category_id":%q,"amount_base_minor":-100000}]}`,
		account.ID, category.ID,
	), nil)
	if status != http.StatusConflict {
		t.Fatalf("unbalanced split = %d, want %d", status, http.StatusConflict)
	}
}

// Workspace isolation is a security boundary, so it is checked against a real second tenant
// rather than a stubbed authorizer: every workspace-scoped route must refuse a non-member.
func TestJourneyRefusesAccessAcrossWorkspaces(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("insider@example.com", "Atay Family")
	journey.token = owner.BearerToken
	workspaceID := owner.Workspaces[0].ID

	var account struct {
		ID string `json:"id"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Checking","type":"bank","currency":"TRY"}`, &account)

	// A second registration is a separate tenant with no relationship to the first.
	outsider := journey.register("outsider@example.com", "Other Household")
	journey.token = outsider.BearerToken

	for _, probe := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/accounts", ""},
		{http.MethodGet, "/accounts/" + account.ID, ""},
		{http.MethodPost, "/accounts", `{"name":"Theirs","type":"cash","currency":"TRY"}`},
		{http.MethodPut, "/accounts/" + account.ID, `{"name":"Theirs","type":"cash","currency":"TRY"}`},
		{http.MethodDelete, "/accounts/" + account.ID, ""},
		{http.MethodGet, "/categories", ""},
		{http.MethodGet, "/transactions", ""},
		{http.MethodPost, "/transactions", `{"kind":"standard","status":"posted","transaction_date":"2026-08-18","entries":[],"allocations":[]}`},
		{http.MethodGet, "/budgets?month=2026-08", ""},
		{http.MethodGet, "/financial-projection?from_date=2026-08-01&to_date=2026-08-31", ""},
		{http.MethodGet, "/members", ""},
		{http.MethodGet, "/invitations", ""},
	} {
		t.Run(probe.method+probe.path, func(t *testing.T) {
			status := journey.do(probe.method, "/v1/workspaces/"+workspaceID+probe.path, probe.body, nil)
			if status != http.StatusForbidden {
				t.Fatalf("status = %d, want %d for a non-member", status, http.StatusForbidden)
			}
		})
	}

	// Exchange rates answer 503 here rather than 403, because this deployment has rate
	// fetching disabled and the feature check precedes the membership check. That leaks
	// nothing: with the feature off every caller sees 503 whether or not they are a member,
	// and with it on the membership check inside the service produces 403 like the routes
	// above. The response varies by server configuration, never by membership.
	if status := journey.do(
		http.MethodGet, "/v1/workspaces/"+workspaceID+"/exchange-rates", "", nil,
	); status != http.StatusServiceUnavailable {
		t.Fatalf("exchange rates = %d, want %d when rate fetching is disabled",
			status, http.StatusServiceUnavailable)
	}
}

// The collaboration journey: invite, accept with the disclosed token, and confirm the new
// member gains exactly the access their role allows and no more.
func TestJourneyInvitesAndGrantsScopedAccess(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("host@example.com", "Atay Family")
	guest := journey.register("guest@example.com", "Guest Household")

	journey.token = owner.BearerToken
	workspaceID := owner.Workspaces[0].ID
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Checking","type":"bank","currency":"TRY"}`, nil)

	var issued struct {
		AcceptanceToken string `json:"acceptance_token"`
		Invitation      struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/invitations",
		`{"email":"guest@example.com","role":"viewer"}`, &issued)
	if len(issued.AcceptanceToken) != 43 {
		t.Fatalf("acceptance token length = %d, want 43", len(issued.AcceptanceToken))
	}

	// Acceptance is authorized by the token alone, not by the invited address, so the guest
	// accepts as themselves.
	journey.token = guest.BearerToken
	var acceptance struct {
		Member struct {
			Role string `json:"role"`
		} `json:"member"`
		Workspace struct {
			ID string `json:"id"`
		} `json:"workspace"`
	}
	journey.mustDo(http.MethodPost, "/v1/invitations/accept",
		fmt.Sprintf(`{"token":%q}`, issued.AcceptanceToken), &acceptance)
	if acceptance.Member.Role != "viewer" || acceptance.Workspace.ID != workspaceID {
		t.Fatalf("acceptance = %#v", acceptance)
	}

	// A viewer reads the workspace.
	var accounts struct {
		Accounts []struct {
			ID string `json:"id"`
		} `json:"accounts"`
	}
	journey.mustDo(http.MethodGet, "/v1/workspaces/"+workspaceID+"/accounts", "", &accounts)
	if len(accounts.Accounts) != 1 {
		t.Fatalf("viewer saw %d accounts, want 1", len(accounts.Accounts))
	}

	// A viewer writes nothing, and cannot see who else has been invited.
	if status := journey.do(http.MethodPost, "/v1/workspaces/"+workspaceID+"/accounts",
		`{"name":"Theirs","type":"cash","currency":"TRY"}`, nil); status != http.StatusForbidden {
		t.Fatalf("viewer create = %d, want %d", status, http.StatusForbidden)
	}
	if status := journey.do(http.MethodGet, "/v1/workspaces/"+workspaceID+"/invitations",
		"", nil); status != http.StatusForbidden {
		t.Fatalf("viewer invitation list = %d, want %d", status, http.StatusForbidden)
	}

	// A single-use token cannot be replayed by a different user.
	third := journey.register("third@example.com", "Third Household")
	journey.token = third.BearerToken
	if status := journey.do(http.MethodPost, "/v1/invitations/accept",
		fmt.Sprintf(`{"token":%q}`, issued.AcceptanceToken), nil); status/100 == 2 {
		t.Fatal("a consumed invitation was accepted a second time")
	}
}

// Removing a member revokes access immediately; a session that was valid a moment ago must
// stop working on that workspace.
func TestJourneyRemovalRevokesAccessImmediately(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("keeper@example.com", "Atay Family")
	guest := journey.register("leaver@example.com", "Guest Household")
	workspaceID := owner.Workspaces[0].ID

	journey.token = owner.BearerToken
	var issued struct {
		AcceptanceToken string `json:"acceptance_token"`
	}
	journey.mustDo(http.MethodPost, "/v1/workspaces/"+workspaceID+"/invitations",
		`{"email":"leaver@example.com","role":"member"}`, &issued)

	journey.token = guest.BearerToken
	journey.mustDo(http.MethodPost, "/v1/invitations/accept",
		fmt.Sprintf(`{"token":%q}`, issued.AcceptanceToken), nil)
	journey.mustDo(http.MethodGet, "/v1/workspaces/"+workspaceID+"/accounts", "", nil)

	journey.token = owner.BearerToken
	journey.mustDo(http.MethodDelete,
		"/v1/workspaces/"+workspaceID+"/members/"+guest.User.ID, "", nil)

	// The guest's session is still valid; only their membership is gone.
	journey.token = guest.BearerToken
	if status := journey.do(http.MethodGet, "/v1/workspaces/"+workspaceID+"/accounts",
		"", nil); status != http.StatusForbidden {
		t.Fatalf("removed member read = %d, want %d", status, http.StatusForbidden)
	}
}

// A workspace must always retain an owner, so the last one cannot leave or be demoted.
func TestJourneyProtectsTheLastOwner(t *testing.T) {
	journey := newJourney(t)
	owner := journey.register("sole@example.com", "Atay Family")
	journey.token = owner.BearerToken
	workspaceID := owner.Workspaces[0].ID

	if status := journey.do(http.MethodDelete,
		"/v1/workspaces/"+workspaceID+"/members/"+owner.User.ID, "", nil); status/100 == 2 {
		t.Fatal("the only owner was allowed to leave")
	}
	if status := journey.do(http.MethodPatch,
		"/v1/workspaces/"+workspaceID+"/members/"+owner.User.ID,
		`{"role":"member"}`, nil); status/100 == 2 {
		t.Fatal("the only owner was allowed to demote themselves")
	}
}
