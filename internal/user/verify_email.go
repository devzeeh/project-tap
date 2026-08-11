package user

import (
	"database/sql"
	"log"
	"net/http"
)

// VerifyEmail verifies the email token and updates the user's email
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var username, pendingEmail string

	// Find the user with this token
	err := h.Store.QueryRowContext(ctx, "SELECT username, pending_email FROM users WHERE email_verification_token = ?", token).Scan(&username, &pendingEmail)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		} else {
			log.Printf("VerifyEmail DB error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if pendingEmail == "" {
		http.Error(w, "No pending email to verify", http.StatusBadRequest)
		return
	}

	// Update the user's email and clear the pending_email and token
	_, err = h.Store.ExecContext(ctx, "UPDATE users SET email = ?, pending_email = NULL, email_verification_token = NULL WHERE username = ?", pendingEmail, username)
	if err != nil {
		log.Printf("VerifyEmail update error: %v", err)
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	// Redirect to profile page with success message (if using query params or similar)
	http.Redirect(w, r, "/u/"+username+"?verified=true", http.StatusSeeOther)
}
