package middleware

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// We need a map to store a rate limiter for each individual IP address
var visitors = make(map[string]*rate.Limiter)
var mu sync.Mutex

// getVisitor checks if an IP already has a limiter. If not, it makes a new one.
func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		// Create a new limiter: allow 1 request per second, with a burst maximum of 3
		limiter = rate.NewLimiter(rate.Every(1*time.Second), 2)
		visitors[ip] = limiter
	}

	return limiter
}

// RateLimitMiddleware acts as a shield in front of your handlers
func RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the user's IP address (simplified for this example)
		ip := r.RemoteAddr

		// Get the rate limiter for this specific IP
		limiter := getVisitor(ip)

		// Ask the limiter if we are allowed to proceed
		if !limiter.Allow() {
			// If they are going too fast, block them!
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests) // 429 Error code
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"message": "Too many login attempts. Please wait a moment and try again.",
			})
			return
		}

		// If they are within the limit, allow the request to pass through to your handler
		next.ServeHTTP(w, r)
	}
}
