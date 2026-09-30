package user

import "net/http"

// RegisterRoutes attaches user and customer HTTP endpoints to the provided ServeMux.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireCustomer func(http.Handler) http.Handler) {
	// Customer Views
	mux.Handle("GET /u/{username}", requireCustomer(http.HandlerFunc(h.ProfileView)))
	mux.Handle("GET /u/{username}/dashboard", requireCustomer(http.HandlerFunc(h.DashboardView)))
	mux.Handle("GET /u/{username}/card", requireCustomer(http.HandlerFunc(h.CardView)))
	mux.Handle("GET /u/{username}/settings", requireCustomer(http.HandlerFunc(h.SettingsView)))
	mux.Handle("GET /u/{username}/topup", requireCustomer(http.HandlerFunc(h.TopUpView)))
	mux.Handle("GET /u/{username}/transaction", requireCustomer(http.HandlerFunc(h.TransactionView)))
	mux.Handle("GET /u/{username}/transactions", requireCustomer(http.HandlerFunc(h.TransactionView)))

	// Customer REST API Endpoints
	mux.Handle("GET /v1/user/{username}", requireCustomer(http.HandlerFunc(h.DashboardHandler)))
	mux.Handle("GET /v1/user/{username}/transactions", requireCustomer(http.HandlerFunc(h.TransactionsJSONHandler)))
	mux.Handle("PATCH /u/{username}/profile/edit", requireCustomer(http.HandlerFunc(h.ProfileEdit)))
	mux.Handle("POST /v1/user/{username}/profile/verify-password", requireCustomer(http.HandlerFunc(h.ProfileVerifyPassword)))
	mux.Handle("PUT /u/{username}/profile/password", requireCustomer(http.HandlerFunc(h.ProfileChangePassword)))
	mux.Handle("POST /v1/user/{username}/card/status", requireCustomer(http.HandlerFunc(h.UpdateCardStatus)))
	mux.Handle("POST /v1/user/{username}/card/replace", requireCustomer(http.HandlerFunc(h.RequestReplacement)))
	mux.Handle("POST /api/topup/create-session/{username}", requireCustomer(http.HandlerFunc(h.CreateXenditInvoice)))
	mux.Handle("POST /v1/user/{username}/topup/checkout", requireCustomer(http.HandlerFunc(h.CreateXenditInvoice)))

	// Public verification & webhooks
	mux.HandleFunc("GET /v1/verify-email", h.VerifyEmail)
	mux.HandleFunc("POST /api/webhooks/xendit/invoice", h.XenditWebhook)
}
