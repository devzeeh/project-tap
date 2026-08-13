package user

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"project-tap/internal/pkg/account"
	jsonwrite "project-tap/internal/pkg/handler"
	smtp "project-tap/internal/pkg/smtpbody"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gomail.v2"
)

// Sentinel errors for password verification
var (
	ErrPasswordLookupFailed = errors.New("Failed to look up password hash")
	ErrIncorrectPassword    = errors.New("Incorrect password")
)

// ProfileUpdateRequest represents the expected payload for updating user profile information
type ProfileUpdateRequest struct {
	Username        string `json:"username,omitempty" db:"username"`
	FullName        string `json:"full_name,omitempty" db:"name"`
	Email           string `json:"email,omitempty" db:"email"`
	Phone           string `json:"phone_number,omitempty" db:"phone_number"`
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password,omitempty"`
	ConfirmPassword string `json:"confirm_password,omitempty"`
}

// verifyCurrentPassword checks the given password against the stored hash for username.
// On success, it returns the stored hash so callers can run further checks
// (e.g. preventing the new password from being the same as the current one).
func (h *Handler) verifyCurrentPassword(ctx context.Context, username, password string) (string, error) {
	var currentHash string
	err := h.Store.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE username = ?", username).Scan(&currentHash)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPasswordLookupFailed, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(password)); err != nil {
		return "", ErrIncorrectPassword
	}

	return currentHash, nil
}

// ProfileView handles the display of the user's profile page
// It retrieves the username from the URL and renders the profile template
func (h *Handler) ProfileView(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Profile view is running...")

	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.GetDashboardUser(username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	}

	// Render the profile template with the username data
	h.Tpl.ExecuteTemplate(w, "profile.html", data)
}
func (h *Handler) ProfileEdit(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}
	ctx := r.Context()

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	// PATCH â€” at least one field required
	if req.FullName == "" && req.Email == "" && req.Phone == "" && req.Username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "At least one field is required",
		})
		return
	}

	// Build query dynamically based on what was sent
	fields := []string{}
	args := []any{}

	var currentEmail, currentName string
	emailChanged := false
	if req.Email != "" {
		err := h.Store.QueryRowContext(ctx, "SELECT email, name FROM users WHERE username = ?", username).Scan(&currentEmail, &currentName)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Failed to get current user data: %v", err)
		} else if req.Email != currentEmail {
			emailChanged = true
		}
	}

	if req.FullName != "" {
		fields = append(fields, "name = ?")
		args = append(args, req.FullName)
		currentName = req.FullName // use updated name for email if changed
	}
	if req.Email != "" && !emailChanged {
		fields = append(fields, "email = ?")
		args = append(args, req.Email)
	} else if req.Email != "" && emailChanged {
		b := make([]byte, 16)
		rand.Read(b)
		token := hex.EncodeToString(b)

		fields = append(fields, "pending_email = ?")
		args = append(args, req.Email)
		fields = append(fields, "email_verification_token = ?")
		args = append(args, token)

		go func(emailTo, emailNew, name, token string) {
			smtpHost := os.Getenv("SMTP_HOST")
			smtpPort := 587
			smtpEmail := os.Getenv("SMTP_EMAIL")
			smtpSender := os.Getenv("SMTP_SENDER")
			smtpPass := os.Getenv("SMTP_PASSWORD")

			if smtpHost == "" || smtpEmail == "" {
				log.Println("SMTP credentials not configured")
				return
			}

			m := gomail.NewMessage()
			m.SetHeader("From", smtpSender+" <"+smtpEmail+">")
			m.SetHeader("To", emailTo)
			m.SetHeader("Subject", "Approve Your Email Change")

			baseURL := os.Getenv("BASE_URL")
			if baseURL == "" {
				baseURL = "http://localhost:3001"
			}
			verifyURL := fmt.Sprintf("%s/v1/verify-email?token=%s", baseURL, token)
			htmlBody := fmt.Sprintf(smtp.EmailVerificationBody(), name, emailNew, verifyURL)
			m.SetBody("text/html", htmlBody)

			d := gomail.NewDialer(smtpHost, smtpPort, smtpEmail, smtpPass)
			if err := d.DialAndSend(m); err != nil {
				log.Printf("Failed to send verification email: %v", err)
			}
		}(currentEmail, req.Email, currentName, token)
	}
	if req.Phone != "" {
		fields = append(fields, "phone_number = ?")
		args = append(args, req.Phone)
	}
	if req.Username != "" {
		fields = append(fields, "username = ?")
		args = append(args, req.Username)
	}

	// Append username as last arg for WHERE clause
	args = append(args, username)

	if len(fields) == 0 {
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
			Success: true,
			Message: "No changes to update",
		})
		return
	}

	query := "UPDATE users SET " + strings.Join(fields, ", ") + " WHERE username = ?"

	_, err := h.Store.ExecContext(ctx, query, args...)
	if err != nil {
		log.Printf("ProfileEdit DB error: %v | query: %s | args: %v", err, query, args)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to update profile",
		})
		return
	}

	msg := "Profile updated successfully"
	if emailChanged {
		msg = "Profile updated. Please check your current email to approve the change."
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: msg,
	})
}

// VerifyPasswordRequest represents the payload for verifying current password
type VerifyPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
}
func (h *Handler) ProfileVerifyPassword(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}
	ctx := r.Context()

	var req VerifyPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	if req.CurrentPassword == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Current password is required",
		})
		return
	}

	_, err := h.verifyCurrentPassword(ctx, username, req.CurrentPassword)
	switch {
	case errors.Is(err, ErrPasswordLookupFailed):
		log.Printf("ProfileVerifyPassword lookup error: %v", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to verify current password",
		})
		return
	case errors.Is(err, ErrIncorrectPassword):
		jsonwrite.WriteJSON(w, http.StatusUnauthorized, jsonwrite.APIResponse{
			Success: false,
			Message: "Current password is incorrect",
		})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Password verified",
	})
}
func (h *Handler) ProfileChangePassword(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	var req ProfileUpdateRequest
	var ctx = r.Context()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" || req.ConfirmPassword == "" {
		log.Printf("One or more password fields are empty for user: %s", username)
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "All password fields are required",
		})
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		log.Printf("New password and confirm password do not match for user: %s", username)
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Password do not match",
		})
		return
	}

	// Verify current password
	log.Printf("Verifying current password for user: %s", username)
	currentHash, err := h.verifyCurrentPassword(ctx, username, req.CurrentPassword)
	switch {
	case errors.Is(err, ErrPasswordLookupFailed):
		log.Printf("Error fetching current password hash: %v", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to verify current password",
		})
		return
	case errors.Is(err, ErrIncorrectPassword):
		log.Printf("Current password verification failed for user: %s", username)
		jsonwrite.WriteJSON(w, http.StatusUnauthorized, jsonwrite.APIResponse{
			Success: false,
			Message: "Current password is incorrect",
		})
		return
	}
	log.Printf("Current password verified for user: %s", username)

	// Prevent setting the same password again
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.NewPassword)); err == nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "New password must be different from current password",
		})
		return
	}

	// Hash the new password using the account package's HashPassword function
	hashedPassword, err := account.HashPassword(req.NewPassword)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to hash password",
		})
		return
	}

	// Save new password hash
	query := "UPDATE users SET password_hash = ? WHERE username = ?"
	_, err = h.Store.ExecContext(ctx, query, hashedPassword, username)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to update password",
		})
		return
	}

	// Respond with success message
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Password updated successfully",
	})
}