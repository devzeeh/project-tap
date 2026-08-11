package admin

import "net/http"

// RegisterRoutes wires all administrative endpoints onto the given mux, wrapping them in an authentication middleware.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/{username}", requireAdmin(http.HandlerFunc(h.AdminDashboardView)))
	mux.Handle("GET /v1/admin/{username}/dashboard-data", requireAdmin(http.HandlerFunc(h.AdminDashboardDataHandler)))
	mux.Handle("GET /admin/{username}/merchants", requireAdmin(http.HandlerFunc(h.MerchantManagementView)))
	mux.Handle("GET /v1/admin/{username}/merchants-data", requireAdmin(http.HandlerFunc(h.MerchantManagementDataHandler)))
	mux.Handle("GET /admin/{username}/terminals", requireAdmin(http.HandlerFunc(h.TerminalRegistryView)))
	mux.Handle("GET /v1/admin/{username}/terminals-data", requireAdmin(http.HandlerFunc(h.TerminalRegistryDataHandler)))
	mux.Handle("GET /v1/admin/{username}/terminals/unassigned", requireAdmin(http.HandlerFunc(h.GetUnassignedTerminalsHandler)))
	mux.Handle("POST /v1/admin/{username}/terminals/add", requireAdmin(http.HandlerFunc(h.AddTerminalHandler)))
	mux.Handle("GET /admin/{username}/terminal-requests", requireAdmin(http.HandlerFunc(h.TerminalRequestsView)))
	mux.Handle("GET /v1/admin/{username}/terminal-requests-data", requireAdmin(http.HandlerFunc(h.TerminalRequestsDataHandler)))
	mux.Handle("POST /v1/admin/{username}/terminal-requests/{id}/approve", requireAdmin(http.HandlerFunc(h.ApproveTerminalRequestHandler)))
	mux.Handle("POST /v1/admin/{username}/terminal-requests/{id}/reject", requireAdmin(http.HandlerFunc(h.RejectTerminalRequestHandler)))
	mux.Handle("GET /admin/{username}/settings", requireAdmin(http.HandlerFunc(h.SystemSettingsView)))
	mux.Handle("GET /admin/{username}/transactions", requireAdmin(http.HandlerFunc(h.TransactionsView)))
	mux.Handle("GET /v1/admin/{username}/transactions", requireAdmin(http.HandlerFunc(h.AllTransactionsJSONHandler)))
	mux.Handle("POST /v1/admin/{username}/merchants/add", requireAdmin(http.HandlerFunc(h.AddMerchantHandler)))
	mux.Handle("GET /admin/{username}/merchants/{id}", requireAdmin(http.HandlerFunc(h.MerchantInfoView)))
	mux.Handle("GET /v1/admin/{username}/merchants/{id}/data", requireAdmin(http.HandlerFunc(h.MerchantInfoDataHandler)))
	mux.Handle("POST /v1/admin/{username}/merchants/{id}/approve", requireAdmin(http.HandlerFunc(h.ApproveMerchantHandler)))
	mux.Handle("POST /v1/admin/{username}/merchants/{id}/approve-documents", requireAdmin(http.HandlerFunc(h.ApproveMerchantDocumentsHandler)))
	mux.Handle("POST /v1/admin/{username}/merchants/{id}/reject", requireAdmin(http.HandlerFunc(h.RejectMerchantHandler)))
	mux.Handle("POST /v1/admin/{username}/merchants/{id}/suspend", requireAdmin(http.HandlerFunc(h.SuspendMerchantHandler)))
	mux.Handle("DELETE /v1/admin/{username}/merchants/{id}/delete", requireAdmin(http.HandlerFunc(h.DeleteMerchantHandler)))
	mux.Handle("GET /admin/{username}/card-inventory", requireAdmin(http.HandlerFunc(h.CardInventoryView)))
	mux.Handle("GET /v1/admin/{username}/card-inventory-data", requireAdmin(http.HandlerFunc(h.CardInventoryDataHandler)))
	mux.Handle("POST /v1/admin/{username}/cards/{id}/block", requireAdmin(http.HandlerFunc(h.BlockCardHandler)))
	mux.Handle("GET /admin/{username}/addcard", requireAdmin(http.HandlerFunc(h.AddCardsView)))
	mux.Handle("GET /admin/{username}/deactivatecard", requireAdmin(http.HandlerFunc(h.DeactivateView)))
	mux.Handle("POST /v1/admin/{username}/addcardauth", requireAdmin(http.HandlerFunc(h.AddCardHandler)))
	mux.Handle("POST /v1/admin/{username}/deactivatecardauth", requireAdmin(http.HandlerFunc(h.DeactivateCardHanlder)))
	mux.Handle("POST /v1/admin/{username}/deletecardauth", requireAdmin(http.HandlerFunc(h.DeleteCardHandler)))
	mux.Handle("GET /admin/{username}/delete-cards", requireAdmin(http.HandlerFunc(h.DeleteCardView)))
}
