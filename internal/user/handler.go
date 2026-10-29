package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"project-tap/internal/pkg/cache"
	"project-tap/internal/pkg/database"
	jsonwrite "project-tap/internal/pkg/handler"
)

type Handler struct {
	Store database.Store
	Tpl   *template.Template
	svc   *Service
	tpl   *template.Template
	cache *cache.RedisCache
}

func NewHandler(svc *Service, tpl *template.Template, cache *cache.RedisCache) *Handler {
	return &Handler{Tpl: tpl, tpl: tpl, svc: svc, cache: cache}
}

func (h *Handler) getDashboardUserWithCache(r *http.Request, username string) (*DashboardUser, error) {
	cacheKey := cache.UserDashboardKey(username)
	var cached DashboardUser
	if err := h.cache.GetJSON(cacheKey, &cached); err == nil {
		return &cached, nil
	}

	user, err := h.svc.GetDashboardUser(r.Context(), username)
	if err != nil {
		return nil, err
	}

	_ = h.cache.SetJSON(cacheKey, *user, cache.UserDashboardTTL())
	return user, nil
}

// Dashboard

func (h *Handler) DashboardView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	user, err := h.getDashboardUserWithCache(r, username)
	if err != nil {
		log.Printf("DashboardView: failed for username %s: %v", username, err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     *DashboardUser
	}{
		Username: username,
		User:     user,
	}

	if err := h.Tpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		log.Println("DashboardView: error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) DashboardUserHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "username is required"})
		return
	}

	dashboardUser, err := h.getDashboardUserWithCache(r, username)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
			Success: true,
			Message: "Dashboard data retrieved successfully",
			Data:    dashboardUser,
		})
	case errors.Is(err, ErrUserNotFound):

		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "User not found"})
	default:
		log.Printf("DashboardUserHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Error fetching dashboard data"})
	}
}

func (h *Handler) ProfileView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	user, err := h.getDashboardUserWithCache(r, username)
	if err != nil {
		log.Printf("ProfileView: failed for username %s: %v", username, err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     *DashboardUser
	}{Username: username, User: user}

	if err := h.Tpl.ExecuteTemplate(w, "profile.html", data); err != nil {
		log.Println("ProfileView: error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) ProfileEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}

	result, err := h.svc.UpdateProfile(ctx, username, req)
	switch {
	case err == nil:
		h.cache.InvalidateUser(username)
		if req.Username != "" && req.Username != username {
			h.cache.InvalidateUser(req.Username)
		}
		h.cache.InvalidateAdmin()
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: result.Message})
	case errors.Is(err, ErrNoFieldsToUpdate):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "At least one field is required"})
	default:
		log.Printf("ProfileEdit: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to update profile"})
	}
}

func (h *Handler) ProfileVerifyPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")

	var req struct {
		CurrentPassword string `json:"current_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}
	if req.CurrentPassword == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Current password is required"})
		return
	}

	_, err := h.svc.VerifyPassword(ctx, username, req.CurrentPassword)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Password verified"})
	case errors.Is(err, ErrIncorrectPassword):
		jsonwrite.WriteJSON(w, http.StatusUnauthorized, jsonwrite.APIResponse{Success: false, Message: "Current password is incorrect"})
	default:
		log.Printf("ProfileVerifyPassword: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to verify current password"})
	}
}

func (h *Handler) ProfileChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}

	err := h.svc.ChangePassword(ctx, username, req)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Password updated successfully"})
	case errors.Is(err, ErrMissingPasswordField):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "All password fields are required"})
	case errors.Is(err, ErrPasswordMismatch):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Password do not match"})
	case errors.Is(err, ErrIncorrectPassword):
		jsonwrite.WriteJSON(w, http.StatusUnauthorized, jsonwrite.APIResponse{Success: false, Message: "Current password is incorrect"})
	case errors.Is(err, ErrSamePassword):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "New password must be different from current password"})
	case errors.Is(err, ErrPasswordLookupFailed):
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to verify current password"})
	default:
		log.Printf("ProfileChangePassword: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to update password"})
	}
}

// Cards

func (h *Handler) CardView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	user, err := h.getDashboardUserWithCache(r, username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     *DashboardUser
	}{Username: username, User: user}

	if err := h.Tpl.ExecuteTemplate(w, "card.html", data); err != nil {
		log.Println("Cardview: error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) UpdateUserCardStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")

	var req Status
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.svc.UpdateCardStatus(ctx, username, req.Status)
	switch {
	case err == nil:
		h.cache.InvalidateUser(username)
		h.cache.InvalidateAdmin()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	case errors.Is(err, ErrInvalidCardStatus):
		http.Error(w, "Invalid status", http.StatusBadRequest)
	default:
		log.Printf("UpdateCardStatus: failed for username %s: %v", username, err)
		http.Error(w, "Failed to update card status", http.StatusInternalServerError)
	}
}

func (h *Handler) RequestReplacement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")

	result, err := h.svc.RequestCardReplacement(ctx, username)
	switch {
	case err == nil:
		h.cache.InvalidateUser(username)
		h.cache.InvalidateAdmin()
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
			Success: true,
			Message: "Card replacement requested successfully. Card is now locked.",
			Data:    map[string]any{"transaction_id": result.TransactionID},
		})
	case errors.Is(err, ErrUserNotFound):
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "User not found"})
	case errors.Is(err, ErrCardNotFound):
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "Card not found"})
	case errors.Is(err, ErrInsufficientBalance):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: fmt.Sprintf("Insufficient balance. Replacement fee is %.2f PHP.", cardReplacementFee),
		})
	default:
		log.Printf("RequestReplacement: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to process card replacement"})
	}
}

func (h *Handler) TopUpView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	user, err := h.getDashboardUserWithCache(r, username)
	if err != nil {
		log.Printf("TopUpView: failed for username %s: %v", username, err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     *DashboardUser
	}{Username: username, User: user}

	if err := h.Tpl.ExecuteTemplate(w, "customer_topup.html", data); err != nil {
		log.Println("TopUpView: error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) CreateXenditInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")

	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}

	domain := "http://" + os.Getenv("SERVER_PORT") + ":" + os.Getenv("PORT")
	if domain == "http://" {
		domain = "http://127.0.0.1:3000"
	}

	url, err := h.svc.CreateTopUpInvoice(ctx, username, req, domain)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
			Success: true, Message: "Checkout session created successfully",
			Data: map[string]string{"url": url},
		})
	case errors.Is(err, ErrTopUpTooLow):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Amount must be at least 50.00"})
	case errors.Is(err, ErrTopUpTooHigh):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Amount cannot exceed 2,000.00"})
	case errors.Is(err, ErrUnsupportedPaymentMethod):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "QR Ph is temporarily unavailable. Please choose GCash, Maya, or GrabPay."})
	case errors.Is(err, ErrCardNotFound):
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to find card for user"})
	default:
		log.Printf("CreateXenditInvoice: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to create checkout session"})
	}
}

// SaveTopUpToDatabase finalizes a top-up. This must only be called by a
// trusted, signature-verified caller (e.g. the Xendit invoice webhook) —
// never directly from client-facing input.
func (h *Handler) SaveTopUpToDatabase(w http.ResponseWriter, r *http.Request) {
	callbackToken := r.Header.Get("x-callback-token")
	xenditToken := os.Getenv("XENDIT_WEBHOOK_KEY")
	if xenditToken != "" && callbackToken != xenditToken {
		log.Println("Invalid x-callback-token for topup")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()

	var req TopUpRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}

	if err := h.svc.CompleteTopUp(ctx, req); err != nil {
		if errors.Is(err, ErrCardNotFound) {
			jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "Card not found"})
			return
		}
		log.Printf("SaveTopUpToDatabase: failed for card %s: %v", req.CardNumber, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to process top-up"})
		return
	}

	if uname, err := h.svc.GetUsernameByCardNumber(ctx, req.CardNumber); err == nil && uname != "" {
		h.cache.InvalidateUser(uname)
	}
	h.cache.InvalidateAdmin()

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Top-up fully processed and saved"})
}

func (h *Handler) SettingsView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	user, err := h.getDashboardUserWithCache(r, username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     *DashboardUser
	}{
		Username: username,
		User:     user,
	}

	if err := h.Tpl.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Println("TopUpView: error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

}

func (h *Handler) TransactionView(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	user, err := h.getDashboardUserWithCache(r, username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     *DashboardUser
	}{
		Username: username,
		User:     user,
	}

	if err := h.Tpl.ExecuteTemplate(w, "transaction.html", data); err != nil {
		log.Println("SettingView: error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) TransactionsJSONHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Username is required"})
		return
	}

	cacheKey := cache.UserTransactionsKey(username)
	var cached []TxnResponse
	if err := h.cache.GetJSON(cacheKey, &cached); err == nil {
		jsonwrite.WriteJSON(w, http.StatusOK, struct {
			Success      bool          `json:"success"`
			Transactions []TxnResponse `json:"transactions"`
		}{
			Success:      true,
			Transactions: cached,
		})
		return
	}

	transactions, err := h.svc.GetUserTransactions(ctx, username)
	if err != nil {
		log.Printf("TransactionsJSONHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to fetch transactions"})
		return
	}

	_ = h.cache.SetJSON(cacheKey, transactions, cache.UserTransactionsTTL())

	response := struct {
		Success      bool          `json:"success"`
		Transactions []TxnResponse `json:"transactions"`
	}{
		Success:      true,
		Transactions: transactions,
	}

	jsonwrite.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) XenditWebhook(w http.ResponseWriter, r *http.Request) {
	xenditToken := os.Getenv("XENDIT_WEBHOOK_KEY")
	callbackToken := r.Header.Get("x-callback-token")
	if xenditToken != "" && callbackToken != xenditToken {
		log.Println("Invalid x-callback-token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Failed to read body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var payload XenditWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Println("Failed to parse webhook JSON:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	username, err := h.svc.ProcessTopUpWebhook(r.Context(), payload)
	if err != nil {
		log.Printf("XenditWebhook: failed to process %s (status %s): %v", payload.ExternalID, payload.Status, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if username != "" {
		h.cache.InvalidateUser(username)
	}
	h.cache.InvalidateAdmin()

	// Always 200 OK to Xendit
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	username, err := h.svc.VerifyEmail(r.Context(), token)
	switch {
	case err == nil:
		h.cache.InvalidateUser(username)
		http.Redirect(w, r, "/u/"+username+"?verified=true", http.StatusSeeOther)
	case errors.Is(err, ErrInvalidToken):
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
	case errors.Is(err, ErrNoPendingEmail):
		http.Error(w, "No pending email to verify", http.StatusBadRequest)
	default:
		log.Printf("VerifyEmail: failed for token: %v", err)
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
	}
}

// Helper

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data any) {
	if err := h.tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("renderTemplate %s: %v", name, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false, Message: "Internal Server Error",
		})
	}
}
