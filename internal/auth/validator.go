package auth

import (
	"errors"
	"fmt"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateLoginRequest returns a user-friendly error message for the first
// failing field, or ("", true) when validation passes.
func ValidateLoginRequest(req LoginRequest) (string, bool) {
	return validateStruct(req, map[string]string{
		"Identifier": "Please enter a valid email or username.",
		"Password":   "Please enter your password.",
	})
}

// ValidateSignupRequest returns a user-friendly error for the first failing
// field, or ("", true) when validation passes.
/*func ValidateSignupRequest(req SignupRequest) (string, bool) {
	return validateStruct(req, map[string]string{
		"FirstName":     "First name is required.",
		"LastName":      "Last name is required.",
		"Email":         "Please provide a valid email address.",
		"ContactNumber": "Contact number must be exactly 11 digits.",
		"CardNumber":    "Card number must be exactly 16 digits.",
		"Password":      "Password must be at least 8 characters long.",
	})
} */

// ValidateMerchantSignupRequest returns a user-friendly error for the first
// failing field, or ("", true) when validation passes.
/*func ValidateMerchantSignupRequest(req MerchantSignupRequest) (string, bool) {
	return validateStruct(req, map[string]string{
		"BusinessName":    "Business name is required.",
		"BusinessType":    "Business type is required.",
		"BusinessAddress": "Business address is required.",
		"OwnerName":       "Owner name is required.",
		"BusinessPhone":   "Business phone is required.",
		"BusinessEmail":   "Please provide a valid business email address.",
		"Password":        "Password must be at least 6 characters long.",
	})
} */

// ValidatePassword enforces complexity rules: 8+ chars, upper, lower, digit,
// special character.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsNumber(c):
			hasNumber = true
		case unicode.IsPunct(c), unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	switch {
	case !hasUpper:
		return fmt.Errorf("password must contain at least one uppercase letter")
	case !hasLower:
		return fmt.Errorf("password must contain at least one lowercase letter")
	case !hasNumber:
		return fmt.Errorf("password must contain at least one number")
	case !hasSpecial:
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}

// ValidateForgotPasswordRequest returns a user-friendly error for an invalid email, or ("", true) on success.
/* func ValidateForgotPasswordRequest(req ForgotPasswordRequest) (string, bool) {
	return validateStruct(req, map[string]string{
		"Email": "Please enter a valid email address.",
	})
} */

// internal helper

// validateStruct runs go-playground/validator against any struct and maps the
// first failing field name to a human-readable message via fieldMessages.
func validateStruct(s any, fieldMessages map[string]string) (string, bool) {
	err := validate.Struct(s)
	if err == nil {
		return "", true
	}

	msg := "Invalid input provided."
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		if custom, ok := fieldMessages[validationErrs[0].Field()]; ok {
			msg = custom
		}
	}
	return msg, false
}
