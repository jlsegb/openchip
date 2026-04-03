package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/openchip/openchip/api/internal/httpx"
)

type limiter struct {
	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	count   int
	expires time.Time
}

func NewRateLimit(limit int, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	// This limiter is intentionally in-memory for a single-node deployment.
	// Multi-instance production deployments should back this with a shared store such as Redis.
	l := &limiter{windows: map[string]*window{}}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if !l.allow(fmt.Sprintf("%s:%s", r.URL.Path, key), limit) {
				httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *limiter) allow(key string, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.windows[key]
	if !ok || now.After(entry.expires) {
		l.windows[key] = &window{count: 1, expires: now.Add(time.Minute)}
		return true
	}
	if entry.count >= limit {
		return false
	}
	entry.count++
	return true
}
