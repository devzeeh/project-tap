package merchant

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	jsonwrite "project-tap/internal/pkg/handler"
	structs "project-tap/internal/pkg/structs"
)

// Handler holds the HTTP handlers and dependencies for the merchant package.
type Handler struct {
	svc *Service
	tpl *template.Template
}

// NewHandler creates a new instance of Handler with the provided service and template dependencies.
func NewHandler(svc *Service, tpl *template.Template) *Handler {
	return &Handler{svc: svc, tpl: tpl}
}

// Account page

// Renders the merchant_account.html template for the merchant account page.
// It retrieves the username from the request path and passes it to the template for rendering.
func (h *Handler) MerchantAccountView(w http.ResponseWriter, r *http.Request) {
	log.Println("MerchantAccountView running...")
	h.renderTemplate(w, "merchant_account.html", MerchantPageData{Page: "account", Username: r.PathValue("username")})
}

// Handles the GET request to fetch merchant account data
// It retrieves the merchant's account summary, including profile, details, bank details, and documents.
func (h *Handler) GetMerchantDatahandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := r.PathValue("username")
	if username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}

	summary, err := h.svc.GetMerchantSummary(ctx, username)
	if err != nil {
		log.Printf("MerchantAccountHandler: failed for username %s %v", username, err)
		writeErr(w, http.StatusBadRequest, "Error Fetching account profile")
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Account profile retrieved successfully",
		Data:    summary,
	})
}

// UpdateMerhantBankDetails processes the request to update the merchant's settlement bank details.
func (h *Handler) UpdateMerhantBankDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := r.PathValue("username")
	if username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}

	var req BusinessBankDetails
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	err := h.svc.UpdateBankDetails(ctx, username, req)
	switch {
	case err == nil:
		writeErr(w, http.StatusOK, "Bank details updated")
	case errors.Is(err, ErrInvalidBankDetails):
		writeErr(w, http.StatusBadRequest, "All bank details field are required")
	case errors.Is(err, ErrUnsupportedBank):
		writeErr(w, http.StatusBadRequest, "Unsupported bank selected. Please choose a valid bank from the list")
	case errors.Is(err, ErrMerchantNotFound):
		writeErr(w, http.StatusInternalServerError, "Merchant not found")
	default:
		log.Printf("UpdateBankDetails: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to update bank details"})
	}
}

// UploadDocument handles the multipart form submission for merchant KYC document uploads.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Username required"})
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "File too large"})
		return
	}

	file, fileHeader, err := r.FormFile("document")
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Failed to read file"})
		return
	}
	defer file.Close()

	err = h.svc.UploadDocument(ctx, DocumentUpload{
		Username:    username,
		DocType:     r.FormValue("document_type"),
		File:        file,
		Filename:    fileHeader.Filename,
		Size:        fileHeader.Size,
		ContentType: fileHeader.Header.Get("Content-Type"),
	})

	switch {
	case err == nil:
		writeErr(w, http.StatusOK, "File uploaded successfully")
	case errors.Is(err, ErrFileTooLarge):
		writeErr(w, http.StatusBadRequest, "File too large. Maximum size is 5MB.")
	case errors.Is(err, ErrInvalidFileType):
		writeErr(w, http.StatusBadRequest, "Invalid file format. Only pictures and PDF are allowed.")
	case errors.Is(err, ErrMerchantNotFound):
		writeErr(w, http.StatusBadRequest, "Merchant not found")
	case errors.Is(err, ErrStorageNotReady):
		log.Println("UploadDocument: storage service not initialized")
		writeErr(w, http.StatusInternalServerError, "Storage configuration error")
	default:
		log.Printf("UploadDocument: failed for username %s: %v", username, err)
		writeErr(w, http.StatusInternalServerError, "Failed to upload file")
	}
}

// MerchantDashboardView renders the merchant_dashboard.html template for the merchant.
func (h *Handler) MerchantDashboardView(w http.ResponseWriter, r *http.Request) {
	log.Println("MerchantAccountView running...")
	h.renderTemplate(w, "merchant_account.html", MerchantPageData{Page: "dashboard", Username: r.PathValue("username")})
}

// MerchantDashboardData retrieves and returns the dashboard summary metrics as a JSON response.
func (h *Handler) MerchantDashboardData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "username is required"})
		return
	}

	summary, err := h.svc.GetDashboard(ctx, username)
	if err != nil {
		log.Printf("MerchantDashboardDataHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Error fetching dashboard data"})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Dashboard data retrieved successfully",
		Data:    summary,
	})
}

// IncomeHandler fetches the merchant's income statistics and history, returning it as JSON.
func (h *Handler) IncomeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "username is required"})
		return
	}

	income, err := h.svc.GetIncome(ctx, username)
	if err != nil {
		log.Printf("IncomeHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Error fetching income data"})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Income data retrieved successfully",
		Data:    income,
	})
}

// WithdrawHandler processes a merchant's request to withdraw funds to their settlement bank.
func (h *Handler) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Username is required"})
		return
	}

	var req WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}

	result, err := h.svc.Withdraw(ctx, username, req.Amount)
	switch {
	case err == nil:
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
			Success: true, Message: "Withdrawal is being processed",
			Data: map[string]any{"transaction_id": result.TransactionID, "amount": result.Amount, "status": result.Status},
		})
	case errors.Is(err, ErrInvalidAmount):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Withdrawal amount must be greater than ₱0.00"})
	case errors.Is(err, ErrMerchantNotFound):
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{Success: false, Message: "Merchant not found"})
	case errors.Is(err, ErrBankNotConfigured):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Please set up your settlement bank account details in your profile before withdrawing."})
	case errors.Is(err, ErrBelowMinimum):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Minimum withdrawal amount is ₱500.00."})
	case errors.Is(err, ErrDailyLimitExceeded):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: err.Error()})
	case errors.Is(err, ErrInsufficientBalance):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: err.Error()})
	case errors.Is(err, ErrUnmappedBank):
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Your selected bank is not currently supported for withdrawals."})
	default:
		log.Printf("WithdrawHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to process withdrawal"})
	}
}

// RequestTerminalHandler processes a merchant's request to allocate a new payment terminal.
func (h *Handler) RequestTerminalHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Username required"})
		return
	}

	var payload struct {
		TerminalSN string `json:"terminal_sn"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Invalid request payload"})
		return
	}

	_, err := h.svc.RequestTerminal(ctx, username, payload.TerminalSN, payload.Notes)
	if err != nil {
		if errors.Is(err, ErrMerchantNotFound) {
			jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "Merchant not found"})
			return
		}
		log.Printf("RequestTerminalHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Failed to create terminal request"})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Terminal request submitted",
		Data: structs.Terminal{TerminalSN: payload.TerminalSN,},
	})
}

// MerchantTransactionsView renders the merchant_transactions.html template.
func (h *Handler) MerchantTransactionsView(w http.ResponseWriter, r *http.Request) {
	data := MerchantPageData{Page: "transactions", Username: r.PathValue("username")}
	if err := h.tpl.ExecuteTemplate(w, "merchant_transactions.html", data); err != nil {
		log.Println("Error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// TransactionHandler fetches a filtered list of the merchant's transactions and returns it as JSON.
func (h *Handler) TransactionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{Success: false, Message: "username is required"})
		return
	}

	filter := TransactionFilter{
		Search: r.URL.Query().Get("search"),
		Type:   r.URL.Query().Get("type"),
		Sort:   r.URL.Query().Get("sort"),
		Limit:  100,
	}

	transactions, err := h.svc.GetTransactions(ctx, username, filter)
	if err != nil {
		log.Printf("TransactionHandler: failed for username %s: %v", username, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{Success: false, Message: "Error fetching transactions"})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Transactions fetched successfully", Data: transactions})
}

// XenditDisbursementWebhook processes asynchronous webhook callbacks from Xendit regarding payout updates.
func (h *Handler) XenditDisbursementWebhook(w http.ResponseWriter, r *http.Request) {
	xenditToken := os.Getenv("XENDIT_WEBHOOK_KEY")
	callbackToken := r.Header.Get("x-callback-token")
	if xenditToken != "" && callbackToken != xenditToken {
		log.Println("Invalid x-callback-token for disbursement")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Failed to read body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var payload XenditPayoutWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Println("Failed to parse webhook JSON:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := h.svc.ProcessDisbursementWebhook(r.Context(), payload.Data.ReferenceID, payload.Event, payload.Data.FailureCode); err != nil {
		log.Println("Failed to process disbursement webhook:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Response helper

// renderTemplate is a helper function to render HTML templates and handle errors gracefully.
// It takes the ResponseWriter, template name, and data to be passed to the template.
// If an error occurs during template execution, it logs the error and sends a 500 Internal Server Error response.
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data any) {
	if err := h.tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("renderTemplate %s: %v", name, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false, Message: "Internal Server Error",
		})
	}
}

// writeErr is a helper function to send JSON error responses in a consistent format.
// Returns a JSON response with the specified HTTP status code and error message.
//
// `Sucess: false, Message: msg`
func writeErr(w http.ResponseWriter, status int, msg string) {
	jsonwrite.WriteJSON(w, status, jsonwrite.APIResponse{
		Success: false,
		Message: msg,
	})
}
