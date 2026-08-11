package admin

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	structs "project-tap/internal/pkg/structs"
)

var Validate = validator.New()

// ValidateAddMerchantRequest returns a user-friendly error for the first
// failing field, or ("", true) when validation passes.
func ValidateAddMerchantRequest(req AddMerchantRequest, index int) (string, bool) {
	err := Validate.Struct(req)
	if err == nil {
		return "", true
	}

	msg := fmt.Sprintf("Validation failed on merchant #%d", index+1)
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fieldMessages := map[string]string{
			"BusinessName":      "Business name is required",
			"BusinessType":      "Business type is required",
			"BusinessAddress":   "Business address is required",
			"OwnerName":         "Owner name is required",
			"BusinessEmail":     "A valid business email is required",
			"BusinessPhone":     "Business phone number is required",
			"SettlementName":    "Settlement name is required",
			"SettlementAccount": "Settlement account number is required",
			"SettlementBank":    "Settlement bank name is required",
			"TerminalSN":        "Terminal serial number is required",
		}
		if custom, ok := fieldMessages[validationErrs[0].Field()]; ok {
			msg = fmt.Sprintf("Merchant #%d: %s", index+1, custom)
		}
	}
	return msg, false
}

// ValidateAddCardRequest returns a user-friendly error or ("", true).
func ValidateAddCardRequest(req structs.CardData) (string, bool) {
	err := Validate.Struct(req)
	if err == nil {
		return "", true
	}

	msg := "Invalid input provided."
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fieldMessages := map[string]string{
			"CardUID": "Card UID is required.",
			"Balance": "Initial amount is required and cannot be negative.",
		}
		if custom, ok := fieldMessages[validationErrs[0].Field()]; ok {
			msg = custom
		}
	}
	return msg, false
}

// ValidateDeactivateCardRequest returns a user-friendly error or ("", true).
func ValidateDeactivateCardRequest(req structs.CardData) (string, bool) {
	err := Validate.Struct(req)
	if err == nil {
		return "", true
	}

	msg := "Invalid input provided."
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fieldMessages := map[string]string{
			"CardNumber": "Card number is required.",
			"CardHolder": "Card holder is required.",
			"CardType":   "Card type is required.",
		}
		if custom, ok := fieldMessages[validationErrs[0].Field()]; ok {
			msg = custom
		}
	}
	return msg, false
}

// ValidateDeleteCardRequest returns a user-friendly error or ("", true).
func ValidateDeleteCardRequest(req structs.CardData) (string, bool) {
	err := Validate.Struct(req)
	if err == nil {
		return "", true
	}

	msg := "Invalid input provided."
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		if validationErrs[0].Field() == "CardNumber" {
			msg = "Card number is required."
		}
	}
	return msg, false
}