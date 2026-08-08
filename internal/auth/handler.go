package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	jsonwrite "project-tap/internal/pkg/handler"
)

type Handler struct {
	svc *Service
}

// NewHandler creates a new instance of Handler with the provided dependencies.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}
func (h *Handler) LoginAuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("LoginAuthHandler: decode error: %v", err)
		writeErr(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if msg, ok := ValidateLoginRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	result, err := h.svc.Login(ctx, req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeErr(w, http.StatusUnauthorized, "Incorrect username or password")
			return
		}
		writeErr(w, http.StatusInternalServerError, "Internal server error during login")
		return
	}

	setAuthCookies(w, result.Tokens.Access, result.Tokens.Refresh)
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.LoginResponse{
		Success:     true,
		Message:     "Login successful",
		ID:          result.ID,
		Username:    result.Username,
		RedirectURL: result.RedirectURL,
		Tokens: struct {
			Access  string `json:"access"`
			Refresh string `json:"refresh"`
		}{
			Access:  result.Tokens.Access,
			Refresh: result.Tokens.Refresh,
		},
	})
}

// Cookie helpers

// setAuthCookies attaches the JWT and refresh tokens to the HTTP response.
func setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    accessToken,
		MaxAge:   int(AccessTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		MaxAge:   int(RefreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

// Response helper

// writeErr is a helper function to send JSON error responses.
func writeErr(w http.ResponseWriter, status int, msg string) {
	jsonwrite.WriteJSON(w, status, jsonwrite.APIResponse{
		Success: false,
		Message: msg,
	})
}
