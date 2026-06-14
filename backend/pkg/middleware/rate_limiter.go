package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
}

func newIPRateLimiter(rps rate.Limit, burst int) *ipRateLimiter {
	l := &ipRateLimiter{
		limiters: make(map[string]*limiterEntry),
		rps:      rps,
		burst:    burst,
	}
	go l.cleanup()
	return l
}

func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.RLock()
	entry, exists := l.limiters[ip]
	l.mu.RUnlock()

	if exists {
		l.mu.Lock()
		entry.lastSeen = time.Now()
		l.mu.Unlock()
		return entry.limiter
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists = l.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(l.rps, l.burst)
		entry = &limiterEntry{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		l.limiters[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}

	return entry.limiter
}

func (l *ipRateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		l.mu.Lock()
		for ip, entry := range l.limiters {
			if time.Since(entry.lastSeen) > 10*time.Minute {
				delete(l.limiters, ip)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimiter — 60 req/sec, burst 60 (global default)
func RateLimiter() func(http.Handler) http.Handler {
	limiter := newIPRateLimiter(rate.Limit(60), 60)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := utils.GetClientIP(r)
			lim := limiter.getLimiter(ip)
			if !lim.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AuthRateLimiter — 5 req/min, burst 5 (anti brute-force)
func AuthRateLimiter() func(http.Handler) http.Handler {
	limiter := newIPRateLimiter(rate.Every(12*time.Second), 5)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := utils.GetClientIP(r)
			lim := limiter.getLimiter(ip)
			if !lim.Allow() {
				http.Error(w, "too many authentication attempts, try again later", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
