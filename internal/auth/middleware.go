package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type authVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	authVisitors = make(map[string]*authVisitor)
	authMu       sync.Mutex
)

func init() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			authMu.Lock()
			for ip, v := range authVisitors {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(authVisitors, ip)
				}
			}
			authMu.Unlock()
		}
	}()
}

func getAuthVisitor(ip string) *rate.Limiter {
	authMu.Lock()
	defer authMu.Unlock()

	v, exists := authVisitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Every(1*time.Second), 5)
		authVisitors[ip] = &authVisitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func extractAuthIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware rejects requests that exceed the per-IP rate limit with a 429 JSON response.
func RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractAuthIP(r)
		limiter := getAuthVisitor(ip)

		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"message": "Too many login attempts. Please wait a moment and try again.",
			})
			return
		}
		next(w, r)
	}
}