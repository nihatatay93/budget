package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a per-client token bucket held in memory.
//
// In memory rather than shared: the deployment this targets is a single application process
// next to PostgreSQL, and a shared limiter would mean adding infrastructure the project
// deliberately avoids. A multi-instance deployment gets a limit per instance, which is
// weaker but never wrong.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens restored per second
	burst   float64
	idle    time.Duration // how long an untouched bucket is kept
	now     func() time.Time
	// maxBuckets caps memory when a caller rotates through client addresses.
	maxBuckets int
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func newRateLimiter(perMinute int, burst int, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{
		buckets:    map[string]*bucket{},
		rate:       float64(perMinute) / 60,
		burst:      float64(burst),
		idle:       10 * time.Minute,
		now:        now,
		maxBuckets: 10000,
	}
}

// allow consumes a token for the key, reporting whether the request may proceed.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	moment := l.now()
	existing, found := l.buckets[key]
	if !found {
		l.evictLocked(moment)
		l.buckets[key] = &bucket{tokens: l.burst - 1, lastSeen: moment}
		return true
	}

	elapsed := moment.Sub(existing.lastSeen).Seconds()
	existing.tokens = min(l.burst, existing.tokens+elapsed*l.rate)
	existing.lastSeen = moment
	if existing.tokens < 1 {
		return false
	}
	existing.tokens--
	return true
}

// evictLocked drops idle buckets, and if the map is still at its cap drops the least recently
// seen entry so a caller cycling addresses cannot grow it without bound.
func (l *rateLimiter) evictLocked(moment time.Time) {
	for key, value := range l.buckets {
		if moment.Sub(value.lastSeen) > l.idle {
			delete(l.buckets, key)
		}
	}
	if len(l.buckets) < l.maxBuckets {
		return
	}
	var oldestKey string
	var oldest time.Time
	for key, value := range l.buckets {
		if oldestKey == "" || value.lastSeen.Before(oldest) {
			oldestKey, oldest = key, value.lastSeen
		}
	}
	delete(l.buckets, oldestKey)
}

// clientAddress identifies the caller for rate limiting.
//
// A forwarded header is honoured only when the immediate peer is a configured proxy.
// Trusting it otherwise would let any caller reset their own limit by inventing an address,
// which is worse than having no limit at all because it looks like protection.
func clientAddress(r *http.Request, trusted []*net.IPNet) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if len(trusted) == 0 || !trustedIP(peer, trusted) {
		return peer
	}
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return peer
	}
	// Walk right to left, skipping hops we run, and stop at the first address we did not.
	hops := strings.Split(forwarded, ",")
	for index := len(hops) - 1; index >= 0; index-- {
		candidate := strings.TrimSpace(hops[index])
		if candidate == "" {
			continue
		}
		if trustedIP(candidate, trusted) {
			continue
		}
		return candidate
	}
	return peer
}

func trustedIP(value string, trusted []*net.IPNet) bool {
	address := net.ParseIP(value)
	if address == nil {
		return false
	}
	for _, network := range trusted {
		if network.Contains(address) {
			return true
		}
	}
	return false
}

// rateLimitMiddleware throttles the endpoints that accept credentials.
//
// Only those: a signed-in user listing their accounts is not a guessing attempt, and
// throttling ordinary reads would degrade the product to defend nothing.
func rateLimitMiddleware(
	limiter *rateLimiter, trusted []*net.IPNet, next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limiter == nil || !credentialEndpoint(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !limiter.allow(clientAddress(r, trusted)) {
			w.Header().Set("Retry-After", "60")
			writeError(w, r, http.StatusTooManyRequests, "rate_limited",
				"Too many attempts. Wait a minute and try again.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// credentialEndpoint reports whether a request can be used to guess a secret.
func credentialEndpoint(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/v1/auth/login", "/v1/auth/register", "/v1/invitations/accept":
		return true
	default:
		return false
	}
}
