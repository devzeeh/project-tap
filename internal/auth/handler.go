package auth

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	jsonwrite "project-tap/internal/pkg/handler"
)

// ---------------------------------------------------------------------------
// HTTP Request Structs
// ---------------------------------------------------------------------------

// LoginRequest represents the payload for user authentication.
type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // email, username, or phone
	Password   string `json:"password" validate:"required"`
}

// SignupRequest represents the payload for creating a new customer account.
type SignupRequest struct {
	FirstName     string `json:"first_name" validate:"required"`
	LastName      string `json:"last_name" validate:"required"`
	CardNumber    string `json:"card_number" validate:"required,numeric,len=16"`
	Password      string `json:"password" validate:"required,min=8"`
	Email         string `json:"email" validate:"required,email"`
	ContactNumber string `json:"contact_number" validate:"required,numeric,len=11"`
}

// CheckDetailsRequest represents the payload for verifying email and contact number uniqueness.
type CheckDetailsRequest struct {
	Email         string `json:"email"`
	ContactNumber string `json:"contact_number"`
}

// SignupVerifyOTPRequest represents the payload for verifying the email OTP during signup.
type SignupVerifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

// AdminSignupRequest represents the payload for creating a new admin account.
type AdminSignupRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// MerchantSignupRequest represents the payload for submitting a new merchant application.
type MerchantSignupRequest struct {
	BusinessName     string `json:"businessName" validate:"required"`
	BusinessType     string `json:"businessType" validate:"required"`
	BusinessAddress  string `json:"businessAddress" validate:"required"`
	OwnerName        string `json:"ownerName" validate:"required"`
	BusinessPhone    string `json:"businessPhone" validate:"required"`
	BusinessEmail    string `json:"businessEmail" validate:"required,email"`
	Password         string `json:"password" validate:"required,min=6"`
	BusinessDocument string `json:"businessDocument"`
	BirDocument      string `json:"birDocument"`
	OtherDocument    string `json:"otherDocument"`
}

// ForgotPasswordRequest represents the payload for password reset flows.
type ForgotPasswordRequest struct {
	Email       string `json:"email" validate:"required,email"`
	OTP         string `json:"otp"`
	NewPassword string `json:"new_password"`
}

// Handler holds HTTP handlers for the auth package.
type Handler struct {
	svc *Service
	tpl *template.Template
}

// NewHandler creates a new instance of Handler with the provided dependencies.
func NewHandler(svc *Service, tpl *template.Template) *Handler {
	return &Handler{svc: svc, tpl: tpl}
}

// Login

// LoginView renders the login HTML template.
func (h *Handler) LoginView(w http.ResponseWriter, r *http.Request) {
	h.tpl.ExecuteTemplate(w, "login.html", nil)
}
func (h *Handler) LoginAuthHandler(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.svc.Login(req)
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
	})
}

// Logout

// LogoutHandler clears the authentication cookies and redirects the user.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	clearAuthCookies(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Refresh token
func (h *Handler) RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "Unauthorized: missing refresh token")
		return
	}

	claims, err := ValidateJWT(cookie.Value)
	if err != nil || claims.Subject != "refresh" {
		log.Printf("RefreshTokenHandler: invalid token: %v", err)
		writeErr(w, http.StatusUnauthorized, "Unauthorized: invalid or expired refresh token")
		return
	}

	access, refresh, err := GenerateTokens(claims.UserID, claims.Role)
	if err != nil {
		log.Printf("RefreshTokenHandler: token generation failed: %v", err)
		writeErr(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	setAuthCookies(w, access, refresh)
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Token refreshed successfully",
	})
}

// Customer signup

// SignupView renders the customer signup HTML template.
func (h *Handler) SignupView(w http.ResponseWriter, r *http.Request) {
	h.tpl.ExecuteTemplate(w, "signup.html", nil)
}
func (h *Handler) SignupSendOTP(w http.ResponseWriter, r *http.Request) {
	var req CheckDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.svc.SignupSendOTP(req.Email, req.ContactNumber); err != nil {
		switch err.Error() {
		case "email already registered":
			jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
				Success: false, Message: err.Error(), Field: "email",
			})
		case "phone number already registered":
			jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
				Success: false, Message: err.Error(), Field: "phone",
			})
		default:
			writeErr(w, http.StatusInternalServerError, "Failed to send OTP email")
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "OTP sent successfully to your email",
	})
}
func (h *Handler) SignupVerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req SignupVerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid input")
		return
	}

	if err := h.svc.SignupVerifyOTP(req.Email, req.OTP); err != nil {
		status := http.StatusUnauthorized
		if err.Error() == "OTP expired" {
			status = http.StatusGone
		}
		writeErr(w, status, err.Error())
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Email successfully verified",
	})
}
func (h *Handler) CheckCardHandler(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.svc.CheckCard(req.CardNumber); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false, Message: err.Error(), Field: "card",
		})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Card is valid", Field: "card",
	})
}
func (h *Handler) SignupHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("SignupHandler: running")
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Failed to parse JSON request")
		return
	}

	if msg, ok := ValidateSignupRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	if err := h.svc.Signup(r.Context(), req); err != nil {
		writeErr(w, http.StatusInternalServerError, "System error finalizing account creation. Please try again.")
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Account created successfully!",
	})
}

// Admin signup

// AdminSignupView renders the admin signup HTML template.
func (h *Handler) AdminSignupView(w http.ResponseWriter, r *http.Request) {
	h.tpl.ExecuteTemplate(w, "admin_signup.html", nil)
}
func (h *Handler) AdminSignupHandler(w http.ResponseWriter, r *http.Request) {
	var req AdminSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if err := h.svc.AdminSignup(r.Context(), req); err != nil {
		switch err.Error() {
		case "all fields are required":
			writeErr(w, http.StatusBadRequest, err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, "Database error creating account. Email or Username might be taken.")
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Super Admin account created successfully!",
	})
}

// Merchant signup

// MerchantSignupView renders the merchant application HTML template.
func (h *Handler) MerchantSignupView(w http.ResponseWriter, r *http.Request) {
	h.tpl.ExecuteTemplate(w, "merchant_signup.html", nil)
}
func (h *Handler) MerchantSignupHandler(w http.ResponseWriter, r *http.Request) {
	var req MerchantSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Failed to parse JSON request")
		return
	}

	if msg, ok := ValidateMerchantSignupRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	if err := h.svc.MerchantSignup(r.Context(), req); err != nil {
		switch err.Error() {
		case "email already registered":
			writeErr(w, http.StatusBadRequest, err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Merchant application submitted successfully",
	})
}

// Forgot / reset password

// ForgotPasswordView renders the forgot password HTML template.
func (h *Handler) ForgotPasswordView(w http.ResponseWriter, r *http.Request) {
	h.tpl.ExecuteTemplate(w, "forgot-password.html", nil)
}
func (h *Handler) ForgotPasswordSendOTP(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid input")
		return
	}

	if msg, ok := ValidateForgotPasswordRequest(req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	// Errors are suppressed intentionally to prevent email enumeration.
	if err := h.svc.ForgotPasswordSendOTP(r.Context(), req.Email); err != nil {
		log.Printf("ForgotPasswordSendOTP: %v", err)
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "If the email is found, an OTP has been sent.",
	})
}
func (h *Handler) ForgotPasswordVerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid input")
		return
	}

	if err := h.svc.ForgotPasswordVerifyOTP(req.Email, req.OTP); err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "OTP verified",
	})
}
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid input")
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Email, req.OTP, req.NewPassword); err != nil {
		switch err.Error() {
		case "invalid OTP", "OTP expired":
			writeErr(w, http.StatusUnauthorized, err.Error())
		default:
			writeErr(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true, Message: "Password updated successfully",
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

// clearAuthCookies removes the JWT and refresh token cookies from the client.
func clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"jwt", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
		})
	}
}

// Response helper

// writeErr is a helper function to send JSON error responses.
func writeErr(w http.ResponseWriter, status int, msg string) {
	jsonwrite.WriteJSON(w, status, jsonwrite.APIResponse{
		Success: false,
		Message: msg,
	})
}
