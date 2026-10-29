package user

import "github.com/shopspring/decimal"

type Status struct {
	Status string `json:"status"`
}

// ProfileUpdateRequest represents the expected payload for updating user profile information
type ProfileUpdateRequest struct {
	Username        string `json:"username,omitempty" db:"username"`
	FullName        string `json:"full_name,omitempty" db:"name"`
	Email           string `json:"email,omitempty" db:"email"`
	Phone           string `json:"phone_number,omitempty" db:"phone_number"`
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password,omitempty"`
	ConfirmPassword string `json:"confirm_password,omitempty"`
}

// struct for topup request only, for api call not for saving in db
type TopUpRequest struct {
	CardNumber    string          `json:"card_number"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentMethod string          `json:"payment_method"`
}

// struct for topup record, for saving in db and for webhook callback processing
// note the fields that match the db schema
// the external_id is encoded in the external_id field of the xendit invoice
// this is necessary because xendit v1 doesn't have a metadata field
type TopUpRecord struct {
	TopupID        string          `json:"topup_id" db:"topup_id"`
	CardNumber     string          `json:"card_number" db:"card_number"`
	Amount         decimal.Decimal `json:"amount" db:"amount"`
	ConvenienceFee decimal.Decimal `json:"convenience_fee" db:"convenience_fee"`
	GatewayCost    decimal.Decimal `json:"gateway_cost" db:"gateway_cost"`
	PaymentMethod  string          `json:"payment_method" db:"payment_method"`
}

type Transaction struct {
	TransactionID   string          `json:"transaction_id"`
	TransactionType string          `json:"transaction_type"`
	TerminalID      string          `json:"terminal_id"`
	Date            string          `json:"date" db:"date"`
	Time            string          `json:"time"`
	Description     string          `json:"description" db:"description"`
	CreatedAt       string          `json:"created_at"`
	Type            string          `json:"type" db:"transaction_type"`
	Amount          float64         `json:"amount" db:"transaction_amount"`
	Status          string          `json:"status" db:"status"`
	MerchantName    string          `json:"merchant_name"`
	BusinessName    string          `json:"business_name"`
	MerchantID      string          `json:"merchant_id"`
	ServiceFee      float64         `json:"service_fee"`
	PointsEarned    decimal.Decimal `json:"points_earned"`
}

// DashboardUser info struct for the user dashboard view
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
	AccountType        string          `db:"account_type" json:"account_type"`
	CardNumber         string          `json:"card_number"`
	CardExpiry         string          `json:"card_expiry"`
	CardStatus         string          `json:"card_status"`
	UserStatus         string          `json:"user_status"`
	RecentTransactions []Transaction   `json:"recent_transactions"` // Add recent transactions to the dashboard response
}

// XenditWebhookPayload represents the expected payload from Xendit Invoice webhook
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
