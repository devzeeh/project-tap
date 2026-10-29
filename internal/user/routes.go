package user

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, requireCustomer func(http.Handler) http.Handler) {
	mux.Handle("GET /u/{username}/dashboard", requireCustomer(http.HandlerFunc(h.DashboardView)))
	mux.Handle("GET /v1/user/{username}", requireCustomer(http.HandlerFunc(h.DashboardUserHandler)))
	mux.Handle("GET /u/{username}", requireCustomer(http.HandlerFunc(h.ProfileView)))
	mux.Handle("PATCH /u/{username}/profile/edit", requireCustomer(http.HandlerFunc(h.ProfileEdit)))
	mux.Handle("POST /v1/user/{username}/profile/verify-password", requireCustomer(http.HandlerFunc(h.ProfileVerifyPassword)))
	mux.Handle("PUT /u/{username}/profile/password", requireCustomer(http.HandlerFunc(h.ProfileChangePassword)))
	mux.Handle("GET /u/{username}/card", requireCustomer(http.HandlerFunc(h.CardView)))
	mux.Handle("POST /v1/user/{username}/card/status", requireCustomer(http.HandlerFunc(h.UpdateUserCardStatus)))
	mux.Handle("POST /v1/user/{username}/card/replace", requireCustomer(http.HandlerFunc(h.RequestReplacement)))
	mux.Handle("GET /u/{username}/topup", requireCustomer(http.HandlerFunc(h.TopUpView)))
	mux.Handle("POST /api/topup/create-session/{username}", requireCustomer(http.HandlerFunc(h.CreateXenditInvoice)))
	mux.Handle("POST /v1/user/{username}/topup/checkout", requireCustomer(http.HandlerFunc(h.CreateXenditInvoice)))
	mux.Handle("GET /u/{username}/settings", requireCustomer(http.HandlerFunc(h.SettingsView)))
	mux.Handle("GET /u/{username}/transaction", requireCustomer(http.HandlerFunc(h.TransactionView)))
	mux.Handle("GET /v1/user/{username}/transactions", requireCustomer(http.HandlerFunc(h.TransactionsJSONHandler)))
	mux.HandleFunc("GET /v1/verify-email", h.VerifyEmail)
	mux.HandleFunc("POST /api/webhooks/xendit/invoice", h.XenditWebhook)
}
