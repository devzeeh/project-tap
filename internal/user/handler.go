package user

import (
	"html/template"
	"net/http"
	"project-tap/internal/middleware"
	"project-tap/internal/pkg/database"
)

type Handler struct {
	Store database.Store
	Tpl   *template.Template
}

func NewHandler(store database.Store, tpl *template.Template) *Handler {
	return &Handler{Store: store, Tpl: tpl}
}

// IsAuthorizedUser verifies that the request context JWT claims match the target username or user_id.
func (h *Handler) IsAuthorizedUser(r *http.Request, targetUsername string) bool {
	claims, ok := middleware.GetUserClaims(r)
	if !ok || claims == nil {
		return false
	}
	if claims.Role == "super_admin" {
		return true
	}

	// Direct match with UserID
	if claims.UserID == targetUsername {
		return true
	}

	// Lookup username for the UserID in claims
	var requestingUsername string
	err := h.Store.QueryRowContext(r.Context(), "SELECT username FROM users WHERE user_id = ?", claims.UserID).Scan(&requestingUsername)
	if err != nil {
		return false
	}

	return requestingUsername == targetUsername
}
