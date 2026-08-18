package httpapi

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testNetworks(t *testing.T, entries ...string) []*net.IPNet {
	t.Helper()
	var networks []*net.IPNet
	for _, entry := range entries {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			t.Fatalf("parse %q: %v", entry, err)
		}
		networks = append(networks, network)
	}
	return networks
}

func TestRateLimiterAllowsBurstThenRefills(t *testing.T) {
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(60, 3, func() time.Time { return moment })

	for attempt := 1; attempt <= 3; attempt++ {
		if !limiter.allow("client") {
			t.Fatalf("attempt %d was refused inside the burst", attempt)
		}
	}
	if limiter.allow("client") {
		t.Fatal("the burst was exceeded without refusal")
	}

	// One token per second at 60 per minute.
	moment = moment.Add(time.Second)
	if !limiter.allow("client") {
		t.Fatal("a token was not restored after a second")
	}
	if limiter.allow("client") {
		t.Fatal("more than the restored token was granted")
	}
}

// Clients are limited independently: one caller exhausting their allowance must not lock
// everyone else out.
func TestRateLimiterKeepsClientsIndependent(t *testing.T) {
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(60, 1, func() time.Time { return moment })

	if !limiter.allow("first") || limiter.allow("first") {
		t.Fatal("the first client's allowance did not behave")
	}
	if !limiter.allow("second") {
		t.Fatal("a second client was refused because of the first")
	}
}

// A caller cycling through addresses must not grow the bucket map without bound.
func TestRateLimiterEvictsIdleAndCapsGrowth(t *testing.T) {
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(60, 1, func() time.Time { return moment })
	limiter.maxBuckets = 8

	for index := range 50 {
		limiter.allow(string(rune('a'+index%26)) + string(rune('0'+index/26)))
	}
	limiter.mu.Lock()
	size := len(limiter.buckets)
	limiter.mu.Unlock()
	if size > limiter.maxBuckets {
		t.Fatalf("bucket count = %d, want at most %d", size, limiter.maxBuckets)
	}

	// Idle buckets are dropped once they age out.
	moment = moment.Add(time.Hour)
	limiter.allow("fresh")
	limiter.mu.Lock()
	remaining := len(limiter.buckets)
	limiter.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("idle buckets survived: %d remain", remaining)
	}
}

// Believing a forwarded header from an untrusted peer would let any caller reset their own
// limit by inventing an address, so it is ignored unless the peer is a configured proxy.
func TestClientAddressTrustsForwardedHeaderOnlyBehindAProxy(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		forwarded string
		trusted   []string
		want      string
	}{
		{
			name: "no proxies configured", remote: "203.0.113.9:5000",
			forwarded: "198.51.100.7", want: "203.0.113.9",
		},
		{
			name: "peer is not a trusted proxy", remote: "203.0.113.9:5000",
			forwarded: "198.51.100.7", trusted: []string{"10.0.0.0/8"}, want: "203.0.113.9",
		},
		{
			name: "peer is a trusted proxy", remote: "10.0.0.5:5000",
			forwarded: "198.51.100.7", trusted: []string{"10.0.0.0/8"}, want: "198.51.100.7",
		},
		{
			// The rightmost address a trusted hop did not add is the real client.
			name: "chained proxies", remote: "10.0.0.5:5000",
			forwarded: "198.51.100.7, 10.0.0.9", trusted: []string{"10.0.0.0/8"},
			want: "198.51.100.7",
		},
		{
			// A spoofed header from behind a trusted proxy still identifies whoever the proxy
			// saw, which is the best available signal.
			name: "trusted proxy with no header", remote: "10.0.0.5:5000",
			trusted: []string{"10.0.0.0/8"}, want: "10.0.0.5",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
			request.RemoteAddr = test.remote
			if test.forwarded != "" {
				request.Header.Set("X-Forwarded-For", test.forwarded)
			}
			got := clientAddress(request, testNetworks(t, test.trusted...))
			if got != test.want {
				t.Fatalf("clientAddress() = %q, want %q", got, test.want)
			}
		})
	}
}

// Only credential endpoints are throttled: a signed-in user reading their accounts is not a
// guessing attempt, and throttling reads would degrade the product to defend nothing.
func TestRateLimitMiddlewareThrottlesOnlyCredentialEndpoints(t *testing.T) {
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(60, 1, func() time.Time { return moment })
	handler := rateLimitMiddleware(limiter, nil, http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	))

	perform := func(method, path string) int {
		request := httptest.NewRequest(method, path, nil)
		request.RemoteAddr = "203.0.113.9:5000"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Code
	}

	if status := perform(http.MethodPost, "/v1/auth/login"); status != http.StatusOK {
		t.Fatalf("first login = %d", status)
	}
	if status := perform(http.MethodPost, "/v1/auth/login"); status != http.StatusTooManyRequests {
		t.Fatalf("second login = %d, want %d", status, http.StatusTooManyRequests)
	}
	// Reads and other routes are untouched even while the credential bucket is empty.
	for _, path := range []string{"/v1/session", "/v1/workspaces/x/accounts", "/healthz"} {
		if status := perform(http.MethodGet, path); status != http.StatusOK {
			t.Fatalf("GET %s = %d, want %d", path, status, http.StatusOK)
		}
	}
	if status := perform(http.MethodPost, "/v1/invitations/accept"); status != http.StatusTooManyRequests {
		t.Fatalf("invitation accept = %d, want throttling", status)
	}
}

func TestRateLimitedResponseIsAProblemDocumentWithRetryAfter(t *testing.T) {
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(60, 1, func() time.Time { return moment })
	handler := requestIDMiddleware(rateLimitMiddleware(limiter, nil, http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	)))

	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		request.RemoteAddr = "203.0.113.9:5000"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusTooManyRequests {
			continue
		}
		if recorder.Header().Get("Retry-After") == "" {
			t.Fatal("no Retry-After on a throttled response")
		}
		if recorder.Header().Get("X-Request-ID") == "" {
			t.Fatal("a throttled response lost its request ID")
		}
	}
}
