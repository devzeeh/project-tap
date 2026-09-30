package user

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"project-tap/internal/middleware"
	"project-tap/internal/pkg/cache"
	jsonwrite "project-tap/internal/pkg/handler"
)

// Handler holds HTTP handlers and dependencies for the user package.
type Handler struct {
	svc   *Service
	tpl   *template.Template
	cache *cache.RedisCache
}

// NewHandler creates a new user HTTP handler.
func NewHandler(svc *Service, tpl *template.Template, cache *cache.RedisCache) *Handler {
	return &Handler{svc: svc, tpl: tpl, cache: cache}
}

// IsAuthorizedUser verifies that the request context JWT claims match the target username.
func (h *Handler) IsAuthorizedUser(r *http.Request, targetUsername string) bool {
	claims, ok := middleware.GetUserClaims(r)
	if !ok || claims == nil {
		log.Printf("[AUTH DENIED] No claims found in request context for target: %q", targetUsername)
		return false
	}
	return h.svc.IsUserAuthorized(r.Context(), claims.UserID, targetUsername, claims.Role == "super_admin")
}

// ---------------------------------------------------------------------------
// HTML Views
// ---------------------------------------------------------------------------

// DashboardView renders the customer dashboard HTML template.
func (h *Handler) DashboardView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.renderTemplate(w, "dashboard.html", struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	})
}

// TransactionView renders the customer transaction history HTML template.
func (h *Handler) TransactionView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.renderTemplate(w, "transaction.html", struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	})
}

// CardView renders the card management HTML template.
func (h *Handler) CardView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.renderTemplate(w, "card.html", struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	})
}

// ProfileView renders the customer profile HTML template.
func (h *Handler) ProfileView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.renderTemplate(w, "profile.html", struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	})
}

// TopUpView renders the customer top-up HTML template.
func (h *Handler) TopUpView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.renderTemplate(w, "customer_topup.html", struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	})
}

// SettingsView renders the customer settings HTML template.
func (h *Handler) SettingsView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.renderTemplate(w, "settings.html", struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	})
}

// ---------------------------------------------------------------------------
// JSON REST Endpoints
// ---------------------------------------------------------------------------

// DashboardHandler returns user dashboard data as JSON with Redis caching.
func (h *Handler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "user is required",
		})
		return
	}

	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	data, err := h.getDashboardData(r.Context(), username)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, data)
}

// TransactionsJSONHandler returns the transactions history as JSON with Redis caching.
func (h *Handler) TransactionsJSONHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Username is required",
		})
		return
	}

	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	cacheKey := cache.UserTransactionsKey(username)
	var cached TransactionsListResponse
	if err := h.cache.GetJSON(cacheKey, &cached); err == nil {
		jsonwrite.WriteJSON(w, http.StatusOK, cached)
		return
	}

	resp, err := h.svc.GetTransactions(r.Context(), username)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Error fetching transactions",
		})
		return
	}

	_ = h.cache.SetJSON(cacheKey, resp, 3*time.Minute)
	jsonwrite.WriteJSON(w, http.StatusOK, resp)
}

// UpdateCardStatus updates the card status (active, inactive, blocked, lost).
func (h *Handler) UpdateCardStatus(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}

	var req UpdateCardStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.svc.UpdateCardStatus(r.Context(), username, req.Status)
	switch {
	case err == nil:
		h.cache.InvalidateUser(username)
		h.cache.InvalidateAdmin()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	case errors.Is(err, ErrInvalidStatus):
		http.Error(w, "Invalid status", http.StatusBadRequest)
	case errors.Is(err, ErrUserNotFound):
		http.Error(w, "User not found", http.StatusNotFound)
	default:
		log.Printf("UpdateCardStatus error: %v", err)
		http.Error(w, "Failed to update card status", http.StatusInternalServerError)
	}
}

// RequestReplacement blocks the existing card and deducts the replacement fee.
func (h *Handler) RequestReplacement(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	err := h.svc.RequestCardReplacement(r.Context(), username)
	switch {
	case err == nil:
		h.cache.InvalidateUser(username)
		h.cache.InvalidateAdmin()
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
			Success: true,
			Message: "Card replacement requested successfully. Card is now locked.",
		})
	case errors.Is(err, ErrUserNotFound):
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "User not found"})
	case errors.Is(err, ErrCardNotFound):
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "Card not found"})
	case errors.Is(err, ErrInsufficientBalance):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Insufficient balance. Replacement fee is 150 PHP.",
		})
	default:
		log.Printf("RequestReplacement error: %v", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to request replacement"})
	}
}

// ProfileEdit updates customer profile details.
func (h *Handler) ProfileEdit(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	emailChanged, err := h.svc.UpdateProfile(r.Context(), username, req)
	if err != nil {
		log.Printf("ProfileEdit error: %v", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to update profile",
		})
		return
	}

	h.cache.InvalidateUser(username)
	if req.Username != "" && req.Username != username {
		h.cache.InvalidateUser(req.Username)
	}
	h.cache.InvalidateAdmin()

	msg := "Profile updated successfully"
	if emailChanged {
		msg = "Profile updated. Please check your current email to approve the change."
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: msg,
	})
}

// ProfileVerifyPassword checks the current user password.
func (h *Handler) ProfileVerifyPassword(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

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

	err := h.svc.VerifyCurrentPassword(r.Context(), username, req.CurrentPassword)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Password verified"})
	case errors.Is(err, ErrIncorrectPassword):
		jsonwrite.WriteJSON(w, http.StatusUnauthorized, jsonwrite.APIResponse{Success: false, Message: "Current password is incorrect"})
	default:
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to verify password"})
	}
}

// ProfileChangePassword updates user password after validating old password.
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	err := h.svc.ChangePassword(r.Context(), username, req.CurrentPassword, req.NewPassword, req.ConfirmPassword)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Password updated successfully"})
	case errors.Is(err, ErrIncorrectPassword):
		jsonwrite.WriteJSON(w, http.StatusUnauthorized, jsonwrite.APIResponse{Success: false, Message: "Current password is incorrect"})
	case errors.Is(err, ErrSamePassword):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "New password must be different from current password"})
	default:
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: err.Error()})
	}
}

// VerifyEmail verifies an email token and redirects to the profile page.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	username, err := h.svc.VerifyEmail(r.Context(), token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	h.cache.InvalidateUser(username)
	http.Redirect(w, r, "/u/"+username+"?verified=true", http.StatusSeeOther)
}

// CreateXenditInvoice creates a Xendit checkout session for card topup.
func (h *Handler) CreateXenditInvoice(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	checkoutURL, err := h.svc.CreateTopUpSession(r.Context(), username, req)
	if err != nil {
		log.Printf("CreateXenditInvoice error: %v", err)
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Checkout session created successfully",
		Data:    map[string]string{"url": checkoutURL},
	})
}

// XenditWebhook receives payment notification webhooks from Xendit.
func (h *Handler) XenditWebhook(w http.ResponseWriter, r *http.Request) {
	xenditToken := os.Getenv("XENDIT_WEBHOOK_KEY")
	callbackToken := r.Header.Get("x-callback-token")

	if xenditToken == "" || subtle.ConstantTimeCompare([]byte(callbackToken), []byte(xenditToken)) != 1 {
		log.Println("Invalid x-callback-token or missing XENDIT_WEBHOOK_KEY")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var payload XenditWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	username, err := h.svc.ProcessXenditWebhook(r.Context(), payload)
	if err != nil {
		log.Printf("XenditWebhook process error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if username != "" {
		h.cache.InvalidateUser(username)
	}
	h.cache.InvalidateAdmin()

	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *Handler) getDashboardData(ctx context.Context, username string) (DashboardUser, error) {
	cacheKey := cache.UserDashboardKey(username)
	var cached DashboardUser
	if err := h.cache.GetJSON(cacheKey, &cached); err == nil {
		return cached, nil
	}

	user, err := h.svc.GetDashboard(ctx, username)
	if err != nil {
		return DashboardUser{}, err
	}

	_ = h.cache.SetJSON(cacheKey, user, 5*time.Minute)
	return user, nil
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data any) {
	if err := h.tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("renderTemplate %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
