package user

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"project-tap/internal/middleware"
	"project-tap/internal/pkg/cache"
	"project-tap/internal/pkg/database"
)

type Handler struct {
	Store database.Store
	Tpl   *template.Template
	Cache *cache.RedisCache
}

func NewHandler(store database.Store, tpl *template.Template, cache *cache.RedisCache) *Handler {
	return &Handler{Store: store, Tpl: tpl, Cache: cache}
}

// IsAuthorizedUser verifies that the request context JWT claims match the target username or user_id.
func (h *Handler) IsAuthorizedUser(r *http.Request, targetUsername string) bool {
	claims, ok := middleware.GetUserClaims(r)
	if !ok || claims == nil {
		log.Printf("[AUTH DENIED] No claims found in request context for target: %q", targetUsername)
		return false
	}
	if claims.Role == "super_admin" {
		return true
	}

	// Direct match with UserID
	if strings.EqualFold(claims.UserID, targetUsername) {
		return true
	}

	// Lookup username for the UserID in claims (matches user_id, id, or username)
	var requestingUsername string
	err := h.Store.QueryRowContext(r.Context(), "SELECT username FROM users WHERE user_id = ? OR id = ? OR username = ?", claims.UserID, claims.UserID, claims.UserID).Scan(&requestingUsername)
	if err != nil {
		log.Printf("[AUTH DENIED] Failed to query username for user_id=%s: %v", claims.UserID, err)
		return false
	}

	if !strings.EqualFold(requestingUsername, targetUsername) {
		log.Printf("[AUTH DENIED] Token user %q (id: %s) does not match requested target %q", requestingUsername, claims.UserID, targetUsername)
		return false
	}

	return true
}
