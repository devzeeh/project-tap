package auth

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	// mux.HandleFunc("GET /login", h.LoginAuthHandler)
	mux.HandleFunc("POST /v1/loginauth", h.LoginAuthHandler)
}