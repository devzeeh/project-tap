package merchant

import "net/http"

// RegisterRoutes attaches merchant-related HTTP endpoints to the provided ServeMux.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireMerchant func(http.Handler) http.Handler) {
	mux.Handle("GET /merchant/{username}/account", requireMerchant(http.HandlerFunc(h.MerchantAccountView)))
	mux.Handle("GET /v1/merchant/{username}/account", requireMerchant(http.HandlerFunc(h.GetMerchantDatahandler)))
	mux.Handle("POST /v1/merchant/{username}/update-bank", requireMerchant(http.HandlerFunc(h.UpdateMerhantBankDetails)))
	mux.Handle("POST /v1/merchant/{username}/upload-document", requireMerchant(http.HandlerFunc(h.UploadDocument)))
	mux.Handle("GET /merchant/{username}/dashboard", requireMerchant(http.HandlerFunc(h.MerchantDashboardView)))
	mux.Handle("GET /v1/merchant/{username}/dashboard", requireMerchant(http.HandlerFunc(h.MerchantDashboardData)))
	mux.Handle("GET /v1/merchant/{username}/incomes", requireMerchant(http.HandlerFunc(h.IncomeHandler)))
	mux.Handle("POST /v1/merchant/{username}/terminals/request", requireMerchant(http.HandlerFunc(h.RequestTerminalHandler)))
	mux.Handle("GET /merchant/{username}/transactions", requireMerchant(http.HandlerFunc(h.MerchantTransactionsView)))
	mux.Handle("GET /v1/merchant/{username}/transactions", requireMerchant(http.HandlerFunc(h.TransactionHandler)))
	mux.Handle("POST /v1/merchant/{username}/withdraw", requireMerchant(http.HandlerFunc(h.WithdrawHandler)))
	mux.HandleFunc("POST /api/webhooks/xendit/disbursement", h.XenditDisbursementWebhook)
}
