package admin

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	jsonwrite "project-tap/internal/pkg/handler"
	structs "project-tap/internal/pkg/structs"

	"github.com/shopspring/decimal"
)

// HTTP Request Structs

// AddMerchantRequest contains the required fields to manually onboard a new merchant.
type AddMerchantRequest struct {
	BusinessName      string `json:"businessName" validate:"required"`
	BusinessType      string `json:"businessType" validate:"required"`
	RegistrationNum   string `json:"registrationNum"`
	BusinessAddress   string `json:"businessAddress" validate:"required"`
	OwnerName         string `json:"ownerName" validate:"required"`
	BusinessEmail     string `json:"businessEmail" validate:"required,email"`
	BusinessPhone     string `json:"businessPhone" validate:"required"`
	CommissionRate    string `json:"commissionRate"`
	SettlementName    string `json:"settlementName" validate:"required"`
	SettlementAccount string `json:"settlementAccount" validate:"required"`
	SettlementBank    string `json:"settlementBank" validate:"required"`
	TerminalSN        string `json:"terminalSn" validate:"required"`
	DeviceName        string `json:"deviceName"`
}

// ApproveMerchantRequest contains the admin-assigned commission rate and terminal for a pending merchant.
type ApproveMerchantRequest struct {
	CommissionRate string `json:"commissionRate"`
	TerminalSn     string `json:"terminalSn"`
}

// RejectMerchantRequest captures the reason for declining a merchant application.
type RejectMerchantRequest struct {
	Reason string `json:"reason"`
}

// SuspendMerchantRequest captures the reason for temporarily deactivating a merchant account.
type SuspendMerchantRequest struct {
	Reason string `json:"reason"`
}

// AddTerminalRequest contains details for adding a new physical payment terminal to the inventory.
type AddTerminalRequest struct {
	TerminalSN string `json:"terminalSn" validate:"required"`
	DeviceName string `json:"deviceName" validate:"required"`
}

// ApproveTerminalRequestPayload captures the terminal assigned to a merchant's hardware request.
type ApproveTerminalRequestPayload struct {
	AssignTerminalSN string `json:"assign_terminal_sn"`
	Notes            string `json:"notes"`
}

// RejectTerminalRequestPayload captures the reason for denying a merchant's terminal request.
type RejectTerminalRequestPayload struct {
	Reason string `json:"reason"`
}

// SimRequest models an emulated point-of-sale transaction for testing card payments.
type SimRequest struct {
	CardNumber string          `json:"card_number"`
	Type       string          `json:"type"`
	Amount     decimal.Decimal `json:"amount"`
	MerchantID string          `json:"merchant_id"`
	Balance    decimal.Decimal `json:"balance"`
	Status     string          `json:"status"`
	UserID     *string         `json:"user_id"`
}

// Handler holds HTTP handlers for the admin package.
type Handler struct {
	svc *Service           // Service layer
	tpl *template.Template // Template layer
}

// NewHandler creates a new admin handler.
// It takes a Service and a template.Template as arguments.
//
// Example: handler := admin.NewHandler(service, template)
func NewHandler(svc *Service, tpl *template.Template) *Handler {
	return &Handler{svc: svc, tpl: tpl}
}

// Dashboard

// Handles admin dashboard view
// Renders the admin dashboard view template
//
// Example: handler.AdminDashboardView(w, r)
func (h *Handler) AdminDashboardView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "admin_dashboard.html", AdminPageData{Page: "dashboard", Username: r.PathValue("username")})
}

// Handles admin dashboard data request
// Fetches admin dashboard data from the database and returns it as JSON
//
// Example: handler.AdminDashboardDataHandler(w, r)
func (h *Handler) AdminDashboardDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("AdminDashboardDataHandler running...")
	data, err := h.svc.GetDashboardData()
	if err != nil {
		log.Printf("AdminDashboardDataHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Admin dashboard data retrieved successfully", Data: data,
	})
}

// Merchants

// Handles merchant management view
// Renders the merchant management view template
//
// Example: handler.MerchantManagementView(w, r)
func (h *Handler) MerchantManagementView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "admin_merchant.html", AdminPageData{Page: "merchants", Username: r.PathValue("username")})
}

// Handles merchant management data request
// Fetches all merchants from the database and returns them as JSON
//
// Example: handler.MerchantManagementDataHandler(w, r)
func (h *Handler) MerchantManagementDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("MerchantManagementDataHandler running...")
	page, limit := parsePagination(r)
	search := r.URL.Query().Get("search")
	sortOrder := r.URL.Query().Get("sort")
	category := r.URL.Query().Get("category")
	status := r.URL.Query().Get("status")

	result, err := h.svc.GetMerchants(page, limit, search, sortOrder, category, status)
	if err != nil {
		log.Printf("MerchantManagementDataHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Error retrieving merchants")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Merchants retrieved successfully", Data: result,
	})
}

// Handles merchant info view
// Renders the merchant info view template
//
// Example: handler.MerchantInfoView(w, r)
func (h *Handler) MerchantInfoView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "merchant_info.html", AdminPageData{Page: "merchants", Username: r.PathValue("username")})
}

// Handles merchant info data request
// Fetches merchant info from the database and returns it as JSON
//
// Example: handler.MerchantInfoDataHandler(w, r)
func (h *Handler) MerchantInfoDataHandler(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("id")
	if merchantID == "" {
		writeErr(w, http.StatusBadRequest, "Merchant ID required")
		return
	}

	m, err := h.svc.GetMerchantByID(merchantID)
	if err != nil {
		if err.Error() == "merchant not found" {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Data: map[string]any{"merchant": m},
	})
}

// Handles merchant approval request
// Validates the request payload and calls the service to approve the merchant
//
// Example: handler.ApproveMerchantHandler(w, r)
func (h *Handler) ApproveMerchantHandler(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("id")
	adminUsername := r.PathValue("username")
	if merchantID == "" {
		writeErr(w, http.StatusBadRequest, "Merchant ID is required")
		return
	}

	var req ApproveMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.svc.ApproveMerchant(r.Context(), merchantID, adminUsername, req); err != nil {
		switch err.Error() {
		case "admin user not found", "merchant not found", "no terminal selected or available", "selected terminal was not found":
			writeErr(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("ApproveMerchantHandler: %v", err)
			writeErr(w, http.StatusInternalServerError, "Failed to approve merchant")
		}
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Merchant approved successfully"})
}

// Handles merchant rejection request
// Validates the request payload and calls the service to reject the merchant
//
// Example: handler.RejectMerchantHandler(w, r)
func (h *Handler) RejectMerchantHandler(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("id")
	if merchantID == "" {
		writeErr(w, http.StatusBadRequest, "Merchant ID is required")
		return
	}

	var req RejectMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.svc.RejectMerchant(merchantID, req); err != nil {
		if err.Error() == "merchant not found" || err.Error() == "rejection reason is required" {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "Failed to reject merchant")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Merchant rejected successfully"})
}

// Handles merchant suspension request
// Validates the request payload and calls the service to suspend the merchant
//
// Example: handler.SuspendMerchantHandler(w, r)
func (h *Handler) SuspendMerchantHandler(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("id")
	if merchantID == "" {
		writeErr(w, http.StatusBadRequest, "Merchant ID is required")
		return
	}

	var req SuspendMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.svc.SuspendMerchant(merchantID, req); err != nil {
		if err.Error() == "merchant not found" || err.Error() == "suspension reason is required" {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "Failed to suspend merchant")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Merchant suspended successfully"})
}

// Handles merchant deletion request
// Validates the request payload and calls the service to delete the merchant
//
// Example: handler.DeleteMerchantHandler(w, r)
func (h *Handler) DeleteMerchantHandler(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("id")
	if merchantID == "" {
		writeErr(w, http.StatusBadRequest, "Merchant ID is required")
		return
	}

	if err := h.svc.DeleteMerchant(merchantID); err != nil {
		if err.Error() == "merchant not found" {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "Failed to delete merchant")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Merchant deleted successfully"})
}

// Handles merchant documents approval request
// Validates the request payload and calls the service to approve the merchant documents
//
// Example: handler.ApproveMerchantDocumentsHandler(w, r)
func (h *Handler) ApproveMerchantDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("id")
	if merchantID == "" {
		writeErr(w, http.StatusBadRequest, "Merchant ID is required")
		return
	}

	if err := h.svc.ApproveMerchantDocuments(merchantID); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Documents approved successfully"})
}

// Handles add merchant request
// Validates the request payload and calls the service to add the merchant
//
// Example: handler.AddMerchantHandler(w, r)
func (h *Handler) AddMerchantHandler(w http.ResponseWriter, r *http.Request) {
	var reqs []AddMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid JSON payload format. Expected array of merchants.")
		return
	}
	if len(reqs) == 0 {
		writeErr(w, http.StatusBadRequest, "No merchants provided.")
		return
	}

	if err := h.svc.AddMerchants(r.Context(), reqs); err != nil {
		log.Printf("AddMerchantHandler: %v", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Successfully onboarded " + strconv.Itoa(len(reqs)) + " merchant(s)",
	})
}

// Terminals

// Handles terminal registry view
// Renders the terminal registry view template
//
// Example: handler.TerminalRegistryView(w, r)
func (h *Handler) TerminalRegistryView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "admin_terminal.html", AdminPageData{Page: "terminals", Username: r.PathValue("username")})
}

// Handles terminal registry data request
// Fetches all terminals from the registry and returns them as JSON
//
// Example: handler.TerminalRegistryDataHandler(w, r)
func (h *Handler) TerminalRegistryDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("TerminalRegistryDataHandler running...")
	page, limit := parsePagination(r)
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	sortOrder := r.URL.Query().Get("sort")

	result, err := h.svc.GetTerminals(page, limit, search, status, sortOrder)
	if err != nil {
		log.Printf("TerminalRegistryDataHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Error retrieving terminals")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Terminals retrieved successfully", Data: result,
	})
}

// Handles add terminal request
// Validates the request payload and calls the service to add the terminal
//
// Example: handler.AddTerminalHandler(w, r)
func (h *Handler) AddTerminalHandler(w http.ResponseWriter, r *http.Request) {
	var req AddTerminalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.TerminalSN == "" || req.DeviceName == "" {
		writeErr(w, http.StatusBadRequest, "Terminal SN and Device Name are required")
		return
	}

	if err := h.svc.AddTerminal(req); err != nil {
		log.Printf("AddTerminalHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to register terminal (SN might already exist)")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Terminal registered to inventory successfully"})
}

// Handles unassigned terminals request
// Fetches all unassigned terminals from the database and returns them as JSON
//
// Example: handler.GetUnassignedTerminalsHandler(w, r)
func (h *Handler) GetUnassignedTerminalsHandler(w http.ResponseWriter, r *http.Request) {
	terminals, err := h.svc.GetUnassignedTerminals()
	if err != nil {
		log.Printf("GetUnassignedTerminalsHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Data: terminals})
}

// Terminal requests

// Handles terminal requests view
// Renders the terminal requests view template
//
// Example: handler.TerminalRequestsView(w, r)
func (h *Handler) TerminalRequestsView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "terminal_requests.html", AdminPageData{Page: "terminal_requests", Username: r.PathValue("username")})
}

// Handles terminal requests data request
// Fetches all terminal requests from the database and returns them as JSON
//
// Example: handler.TerminalRequestsDataHandler(w, r)
func (h *Handler) TerminalRequestsDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("TerminalRequestsDataHandler running...")
	page, limit := parsePagination(r)
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	result, err := h.svc.GetTerminalRequests(page, limit, status, search)
	if err != nil {
		log.Printf("TerminalRequestsDataHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to retrieve terminal requests")
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"message":      "Terminal requests retrieved successfully",
		"data":         result.Requests,
		"total_items":  result.TotalItems,
		"current_page": result.CurrentPage,
		"total_pages":  result.TotalPages,
	})
}

// Handles approve terminal request
// Validates the request payload and calls the service to approve the terminal request
//
// Example: handler.ApproveTerminalRequestHandler(w, r)
func (h *Handler) ApproveTerminalRequestHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("ApproveTerminalRequestHandler running...")
	adminUserID := r.PathValue("username")
	requestID := r.PathValue("id")
	if requestID == "" {
		writeErr(w, http.StatusBadRequest, "Request ID is required")
		return
	}

	var payload ApproveTerminalRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.svc.ApproveTerminalRequest(requestID, adminUserID, payload); err != nil {
		switch err.Error() {
		case "terminal request not found":
			writeErr(w, http.StatusNotFound, err.Error())
		case "only pending requests can be approved", "a terminal must be assigned to approve this request", "terminal is already assigned to another merchant":
			writeErr(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("ApproveTerminalRequestHandler: %v", err)
			writeErr(w, http.StatusInternalServerError, "Failed to approve terminal request")
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "message": "Terminal request approved successfully",
		"data": map[string]any{"request_id": requestID, "terminal_sn": payload.AssignTerminalSN},
	})
}

// Handles reject terminal request
// Validates the request payload and calls the service to reject the terminal request
//
// Example: handler.RejectTerminalRequestHandler(w, r)
func (h *Handler) RejectTerminalRequestHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("RejectTerminalRequestHandler running...")
	adminUserID := r.PathValue("username")
	requestID := r.PathValue("id")
	if requestID == "" {
		writeErr(w, http.StatusBadRequest, "Request ID is required")
		return
	}

	var payload RejectTerminalRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.svc.RejectTerminalRequest(requestID, adminUserID, payload); err != nil {
		switch err.Error() {
		case "terminal request not found":
			writeErr(w, http.StatusNotFound, err.Error())
		case "only pending requests can be rejected":
			writeErr(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("RejectTerminalRequestHandler: %v", err)
			writeErr(w, http.StatusInternalServerError, "Failed to reject terminal request")
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "message": "Terminal request rejected successfully",
		"data": map[string]any{"request_id": requestID, "reason": payload.Reason},
	})
}

// Cards

// Renders the add cards view
//
// Example: handler.AddCardsView(w, r)
func (h *Handler) AddCardsView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "addCards.html", AdminPageData{Page: "addcard", Username: r.PathValue("username")})
}

// Handles add card request
// Validates the request payload and calls the service to add the card
//
// Example: handler.AddCardHandler(w, r)
func (h *Handler) AddCardHandler(w http.ResponseWriter, r *http.Request) {
	var req structs.CardData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if msg, ok := ValidateAddCardRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	if err := h.svc.AddCard(req); err != nil {
		if strings.Contains(err.Error(), "already registered") {
			jsonwrite.WriteJSON(w, http.StatusConflict, jsonwrite.APIResponse{Success: false, Message: err.Error()})
			return
		}
		log.Printf("AddCardHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonwrite.WriteJSON(w, http.StatusCreated, jsonwrite.APIResponse{Success: true, Message: "Card added successfully!"})
}

// Renders the card inventory view
//
// Example: handler.CardInventoryView(w, r)
func (h *Handler) CardInventoryView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "admin_card_inventory.html", AdminPageData{Page: "card-inventory", Username: r.PathValue("username")})
}

// Handles card inventory data request
// Fetches all cards from the inventory and returns them as JSON
//
// Example: handler.CardInventoryDataHandler(w, r)
func (h *Handler) CardInventoryDataHandler(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.GetCardInventory()
	if err != nil {
		log.Printf("CardInventoryDataHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to fetch cards")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, result)
}

// Handles block card request
// Validates the request payload and calls the service to block the card
//
// Example: handler.BlockCardHandler(w, r)
func (h *Handler) BlockCardHandler(w http.ResponseWriter, r *http.Request) {
	cardID := r.PathValue("id")
	if cardID == "" {
		writeErr(w, http.StatusBadRequest, "Card ID is required")
		return
	}

	found, err := h.svc.BlockCard(cardID)
	if err != nil {
		log.Printf("BlockCardHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to block card")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "Card not found or could not be blocked")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Card blocked successfully"})
}

// Renders the deactivate card view
//
// Example: handler.DeactivateView(w, r)
func (h *Handler) DeactivateView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "deactivateCard.html", AdminPageData{Page: "deactivatecard", Username: r.PathValue("username")})
}

// Handles deactivate card request
// Validates the request payload and calls the service to deactivate the card
//
// Example: handler.DeactivateCardHanlder(w, r)
func (h *Handler) DeactivateCardHanlder(w http.ResponseWriter, r *http.Request) {
	var req structs.CardData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if msg, ok := ValidateDeactivateCardRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	ok, err := h.svc.DeactivateCard(strings.TrimSpace(req.CardNumber), strings.TrimSpace(req.CardHolder), strings.TrimSpace(req.CardType))
	if err != nil {
		log.Printf("DeactivateCardHanlder: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to deactivate card.")
		return
	}
	if !ok {
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: false, Message: "Card not found, name/type mismatch, or card is already inactive."})
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Card deactivated successfully!"})
}

// Renders the delete card view
//
// Example: handler.DeleteCardView(w, r)
func (h *Handler) DeleteCardView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "delete_card.html", AdminPageData{Page: "delete-cards", Username: r.PathValue("username")})
}

// Handles delete card request
// Validates the request payload and calls the service to delete the card
//
// Example: handler.DeleteCardHandler(w, r)
func (h *Handler) DeleteCardHandler(w http.ResponseWriter, r *http.Request) {
	var req structs.CardData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if msg, ok := ValidateDeleteCardRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	found, err := h.svc.DeleteCard(strings.TrimSpace(req.CardNumber))
	if err != nil {
		log.Printf("DeleteCardHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to delete card.")
		return
	}
	if !found {
		jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: false, Message: "Card not found."})
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{Success: true, Message: "Card deleted successfully!"})
}

// Transactions

// Renders the transactions view
//
// Example: handler.TransactionsView(w, r)
func (h *Handler) TransactionsView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "transactions.html", AdminPageData{Page: "transactions", Username: r.PathValue("username")})
}

// Gets all transactions
// Example: handler.AllTransactionsJSONHandler(w, r)
func (h *Handler) AllTransactionsJSONHandler(w http.ResponseWriter, r *http.Request) {
	txns, err := h.svc.GetAllTransactions()
	if err != nil {
		log.Printf("AllTransactionsJSONHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to retrieve transactions")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "transactions": txns,
	})
}

// xendit transactions view
func (h *Handler) XenditTransactionsView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "xendit_transactions.html", AdminPageData{Page: "xendit-transactions", Username: r.PathValue("username")})
}

// Gets all Xendit transactions
// Example: handler.AllXenditTransactionsJSONHandler(w, r)
func (h *Handler) AllXenditTransactionsJSONHandler(w http.ResponseWriter, r *http.Request) {
	txns, err := h.svc.GetXenditTransactions()
	if err != nil {
		log.Printf("AllXenditTransactionsJSONHandler: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to fetch Xendit transactions")
		return
	}
	jsonwrite.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "transactions": txns,
	})
}

// Terminal simulation

// Renders the terminal simulation view
// Example: handler.TerminalSimView(w, r)
func (h *Handler) TerminalSimView(w http.ResponseWriter, r *http.Request) {
	merchants := h.svc.GetSimMerchants()
	if err := h.tpl.ExecuteTemplate(w, "terminal_sim.html", struct{ Merchants []SimMerchant }{Merchants: merchants}); err != nil {
		log.Printf("TerminalSimView: %v", err)
	}
}

// Handles terminal simulation transactions
// Validates the request payload and calls the service to process the transaction
//
// Example: handler.TerminalSimTransactionHandler(w, r)
func (h *Handler) TerminalSimTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var req SimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if req.CardNumber == "" || req.Amount.IsZero() || req.Type == "" {
		writeErr(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	serviceFee, err := h.svc.SimTransaction(req)
	if err != nil {
		switch err.Error() {
		case "card not found", "card is not active", "card is not linked to any user", "insufficient balance":
			writeErr(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("TerminalSimTransactionHandler: %v", err)
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "message": "Transaction successful", "service_fee": serviceFee,
	})
}

// Settings (view-only page)
func (h *Handler) SystemSettingsView(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "system_settings.html", AdminPageData{Page: "settings", Username: r.PathValue("username")})
}

// Helpers

// Renders the specified template.
//
// Example: h.renderTemplate(w, "merchant_account.html", MerchantPageData{Page: "account", Username: r.PathValue("username")})
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data any) {
	if err := h.tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("renderTemplate %s: %v", name, err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false, Message: "Internal Server Error",
		})
	}
}

// Writes an error to the response
//
// Example: writeErr(w, http.StatusBadRequest, "Invalid request payload")
func writeErr(w http.ResponseWriter, status int, msg string) {
	jsonwrite.WriteJSON(w, status, jsonwrite.APIResponse{Success: false, Message: msg})
}

// Parses the pagination
func parsePagination(r *http.Request) (page, limit int) {
	page, limit = 1, 10
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	return
}
