package auth

import "net/http"

// RegisterRoutes wires all authentication endpoints onto the given mux.
// Uses Go 1.22+ method-prefixed routing to match your existing style.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	// Views
	mux.HandleFunc("GET /login", h.LoginView)
	mux.HandleFunc("GET /signup", h.SignupView)
	mux.HandleFunc("GET /admin/signup", h.AdminSignupView)
	mux.HandleFunc("GET /merchant/signup", h.MerchantSignupView)
	mux.HandleFunc("GET /forgot-password", h.ForgotPasswordView)

	// Auth actions
	mux.HandleFunc("POST /v1/loginauth", RateLimitMiddleware(h.LoginAuthHandler))
	mux.HandleFunc("GET /logout", h.LogoutHandler)
	mux.HandleFunc("POST /logout", h.LogoutHandler)
	mux.HandleFunc("POST /v1/refresh", h.RefreshTokenHandler)

	// Customer signup flow
	mux.HandleFunc("POST /v1/signup/send-otp", h.SignupSendOTP)
	mux.HandleFunc("POST /v1/signup/verify-otp", h.SignupVerifyOTP)
	mux.HandleFunc("POST /v1/signup/check-card", h.CheckCardHandler)
	mux.HandleFunc("POST /v1/signup", h.SignupHandler)

	// Admin signup
	mux.HandleFunc("POST /v1/admin/signup", h.AdminSignupHandler)

	// Merchant signup
	mux.HandleFunc("POST /v1/merchant/signup", h.MerchantSignupHandler)

	// Forgot / reset password flow
	mux.HandleFunc("POST /v1/forgot-password/send-otp", h.ForgotPasswordSendOTP)
	mux.HandleFunc("POST /v1/forgot-password/verify-otp", h.ForgotPasswordVerifyOTP)
	mux.HandleFunc("POST /v1/forgot-password/reset", h.ResetPassword)
}
