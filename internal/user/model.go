package user

import (
	"errors"

	"github.com/shopspring/decimal"
)

// Sentinel errors
var (
	ErrPasswordLookupFailed = errors.New("failed to look up password hash")
	ErrIncorrectPassword    = errors.New("incorrect password")
	ErrSamePassword         = errors.New("new password must be different from current password")
	ErrUserNotFound         = errors.New("user not found")
	ErrCardNotFound         = errors.New("card not found")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrInvalidStatus        = errors.New("invalid status")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrDatabaseError        = errors.New("database error")
)

const (
	ConvenienceFee      = 15.00
	CardReplacementFee  = 150.00
	MinTopUpAmount      = 50.00
	MaxTopUpAmount      = 2000.00
)

// Transaction represents a single user transaction on the dashboard.
type Transaction struct {
	TransactionID string          `json:"transaction_id"`
	TerminalID    string          `json:"terminal_id"`
	Date          string          `json:"date" db:"date"`
	Time          string          `json:"time"`
	Description   string          `json:"description" db:"description"`
	Type          string          `json:"type" db:"transaction_type"`
	Amount        float64         `json:"amount" db:"transaction_amount"`
	Status        string          `json:"status" db:"status"`
	MerchantName  string          `json:"merchant_name"`
	MerchantID    string          `json:"merchant_id"`
	ServiceFee    float64         `json:"service_fee"`
	PointsEarned  decimal.Decimal `json:"points_earned"`
}

// DashboardUser holds all data rendered on the user dashboard.
type DashboardUser struct {
	ID                 int             `json:"id,omitempty" db:"id"`
	UserID             string          `json:"user_id,omitempty" db:"user_id"`
	Username           string          `json:"username" db:"username"`
	Name               string          `json:"name" db:"name"`
	Email              string          `json:"email" db:"email"`
	PendingEmail       string          `json:"pending_email"`
	Phone              string          `json:"phone" db:"phone"`
	Initials           string          `json:"initials"`
	Balance            float64         `json:"balance" db:"balance"`
	LoyaltyPoints      decimal.Decimal `json:"loyalty_points" db:"loyalty_points"`
	AccountType        string          `json:"account_type" db:"account_type"`
	CardNumber         string          `json:"card_number"`
	CardExpiry         string          `json:"card_expiry"`
	CardStatus         string          `json:"card_status"`
	UserStatus         string          `json:"user_status"`
	RecentTransactions []Transaction   `json:"recent_transactions"`
}

// TxnResponse is the format returned by the transactions list API.
type TxnResponse struct {
	TransactionID string          `json:"transaction_id"`
	TerminalID    string          `json:"terminal_id"`
	Date          string          `json:"date"`
	Time          string          `json:"time"`
	Description   string          `json:"description"`
	Type          string          `json:"type"`
	Amount        float64         `json:"amount"`
	Status        string          `json:"status"`
	MerchantName  string          `json:"merchant_name"`
	MerchantID    string          `json:"merchant_id"`
	ServiceFee    float64         `json:"service_fee"`
	PointsEarned  decimal.Decimal `json:"points_earned"`
	Sender        string          `json:"sender"`
	Receiver      string          `json:"receiver"`
}

// TransactionsListResponse wraps the transactions array.
type TransactionsListResponse struct {
	Success      bool          `json:"success"`
	Transactions []TxnResponse `json:"transactions"`
}

// UpdateCardStatusRequest represents the payload for updating card status.
type UpdateCardStatusRequest struct {
	Status string `json:"status"`
}

// ProfileUpdateRequest represents the payload for updating customer profile details.
type ProfileUpdateRequest struct {
	Username        string `json:"username,omitempty" db:"username"`
	FullName        string `json:"full_name,omitempty" db:"name"`
	Email           string `json:"email,omitempty" db:"email"`
	Phone           string `json:"phone_number,omitempty" db:"phone_number"`
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password,omitempty"`
	ConfirmPassword string `json:"confirm_password,omitempty"`
}

// VerifyPasswordRequest represents the payload for verifying current password.
type VerifyPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
}

// TopUpRequest represents a topup invoice request payload.
type TopUpRequest struct {
	CardNumber    string          `json:"card_number"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentMethod string          `json:"payment_method"`
}

// TopUpRecord represents a record stored in the top_ups table.
type TopUpRecord struct {
	TopupID        string          `json:"topup_id" db:"topup_id"`
	CardNumber     string          `json:"card_number" db:"card_number"`
	Amount         decimal.Decimal `json:"amount" db:"amount"`
	ConvenienceFee decimal.Decimal `json:"convenience_fee" db:"convenience_fee"`
	GatewayCost    decimal.Decimal `json:"gateway_cost" db:"gateway_cost"`
	PaymentMethod  string          `json:"payment_method" db:"payment_method"`
}

// XenditWebhookPayload represents the expected callback body from Xendit.
type XenditWebhookPayload struct {
	ID                     string          `json:"id"`
	ExternalID             string          `json:"external_id"`
	UserID                 string          `json:"user_id"`
	IsHigh                 bool            `json:"is_high"`
	PaymentMethod          string          `json:"payment_method"`
	Status                 string          `json:"status"`
	MerchantName           string          `json:"merchant_name"`
	Amount                 decimal.Decimal `json:"amount"`
	PaidAmount             decimal.Decimal `json:"paid_amount"`
	BankCode               string          `json:"bank_code"`
	PaidAt                 string          `json:"paid_at"`
	PayerEmail             string          `json:"payer_email"`
	Description            string          `json:"description"`
	AdjustedReceivedAmount decimal.Decimal `json:"adjusted_received_amount"`
	FeesPaidAmount         decimal.Decimal `json:"fees_paid_amount"`
	Updated                string          `json:"updated"`
	Created                string          `json:"created"`
	Currency               string          `json:"currency"`
	PaymentChannel         string          `json:"payment_channel"`
	PaymentDestination     string          `json:"payment_destination"`
}
