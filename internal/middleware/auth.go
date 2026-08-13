package middleware

import (
	"context"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
	authentication "project-tap/internal/auth"
	jsonwrite "project-tap/internal/pkg/handler"
)

type contextKey string

const UserClaimsKey contextKey = "user_claims"

// RequireAuth is a middleware that checks for a valid JWT in the Authorization header
// or HttpOnly cookie and ensures the user has one of the allowed roles.
func RequireAuth(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Helper to handle unauthorized/forbidden gracefully depending on endpoint type
			handleAuthError := func(w http.ResponseWriter, r *http.Request, statusCode int, msg string) {
				// Check if it's an API request (starts with /v1/ or /api/)
				if strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/api/") {
					jsonwrite.WriteJSON(w, statusCode, jsonwrite.APIResponse{
						Success: false,
						Message: msg,
					})
				} else {
					// It's a view, redirect to login
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				}
			}

			var claims *authentication.JWTClaims
			var tokenValid bool
			var tokenString string

			// Extract Access Token from Header OR Cookie
			authHeader := r.Header.Get("Authorization")
			if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
				// Extract from Authorization header (API / Thunder Client / cURL)
				tokenString = after
			} else {
				// Fallback: Check if token is in the Cookie (Web Browsers)
				cookie, err := r.Cookie("jwt")
				if err == nil {
					tokenString = cookie.Value
				}
			}

			// If we found a token in either place, validate it
			if tokenString != "" {
				parsedClaims, err := authentication.ValidateJWT(tokenString)
				if err == nil && parsedClaims.Subject == "access" {
					claims = parsedClaims
					tokenValid = true
				} else if err != nil {
					log.Printf("DEBUG: JWT Validation failed: %v", err)
				}
			}

			// 2. If Access Token is missing or invalid, try Silent Refresh
			if !tokenValid {
				refreshCookie, err := r.Cookie("refresh_token")
				if err == nil {
					refreshClaims, err := authentication.ValidateJWT(refreshCookie.Value)
					if err == nil && refreshClaims.Subject == "refresh" {
						// Valid refresh token! Generate new tokens.
						newAccess, newRefresh, err := authentication.GenerateTokens(refreshClaims.UserID, refreshClaims.Role)
						if err == nil {
							// Set new cookies
							http.SetCookie(w, &http.Cookie{
								Name:     "jwt",
								Value:    newAccess,
								Expires:  time.Now().Add(15 * time.Minute),
								HttpOnly: true,
								Secure:   true,
								SameSite: http.SameSiteLaxMode,
								Path:     "/",
							})
							http.SetCookie(w, &http.Cookie{
								Name:     "refresh_token",
								Value:    newRefresh,
								Expires:  time.Now().Add(7 * 24 * time.Hour),
								HttpOnly: true,
								Secure:   true,
								SameSite: http.SameSiteLaxMode,
								Path:     "/",
							})

							// Treat request as valid using the refresh claims
							claims = refreshClaims
							tokenValid = true
						}
					}
				}
			}

			if !tokenValid {
				handleAuthError(w, r, http.StatusUnauthorized, "Unauthorized: Missing or invalid authentication token")
				return
			}

			// 3. Check role-based access control (RBAC)
			roleAllowed := false
			if len(allowedRoles) == 0 {
				// If no specific roles are required, any valid token is sufficient
				roleAllowed = true
			} else {
				if slices.Contains(allowedRoles, claims.Role) {
					roleAllowed = true
				}
			}

			if !roleAllowed {
				handleAuthError(w, r, http.StatusForbidden, "Forbidden: Insufficient privileges")
				return
			}

			// Pass the claims to the next handler via request context
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserClaims retrieves the authenticated user claims from the request context.
func GetUserClaims(r *http.Request) (*authentication.JWTClaims, bool) {
	claims, ok := r.Context().Value(UserClaimsKey).(*authentication.JWTClaims)
	return claims, ok
}
