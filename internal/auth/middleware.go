package auth

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// visitorStore holds a per-IP rate limiter.
// TODO: extract to Redis for multi-instance deployments.
var (
	visitors   = make(map[string]*rate.Limiter)
	visitorsMu sync.Mutex
)

// getVisitor returns the rate limiter for the given IP, creating one if needed.
// Allows 2 requests per second with a burst ceiling of 2.
func getVisitor(ip string) *rate.Limiter {
	visitorsMu.Lock()
	defer visitorsMu.Unlock()

	if limiter, ok := visitors[ip]; ok {
		return limiter
	}
	limiter := rate.NewLimiter(rate.Every(time.Second), 2)
	visitors[ip] = limiter
	return limiter
}

// RateLimitMiddleware rejects requests that exceed the per-IP rate limit with
// a 429 JSON response.
func RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !getVisitor(r.RemoteAddr).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"message": "Too many login attempts. Please wait a moment and try again.",
			})
			return
		}
		next.ServeHTTP(w, r)
	}
}