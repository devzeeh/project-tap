package admin

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
	structs "project-tap/internal/pkg/structs"
)

// ---------------------------------------------------------------------------
// Merchant Models
// ---------------------------------------------------------------------------

// MerchantDetailsData represents the comprehensive profile and settlement information for a merchant.
type MerchantDetailsData struct {
	MerchantID       string             `db:"merchant_id"`
	UserID           string             `db:"user_id"`
	BusinessName     string             `db:"business_name"`
	BusinessType     string             `db:"business_type"`
	RegistrationNum  string             `db:"business_registration_number"`
	BusinessAddress  string             `db:"business_address"`
	City             string             `db:"city"`
	PostalCode       string             `db:"postal_code"`
	OwnerName        string             `db:"owner_name"`
	BusinessEmail    string             `db:"business_email"`
	BusinessPhone    string             `db:"business_phone"`
	Status           string             `db:"status"`
	CommissionRate   float64            `db:"commission_rate"`
	SettlementBank   string             `db:"settlement_bank_name"`
	SettlementName   string             `db:"settlement_account_name"`
	SettlementAcct   string             `db:"settlement_account_number"`
	CreatedAt        string             `db:"created_at"`
	BusinessDocument string             `db:"business_document"`
	BirDocument      string             `db:"bir_document"`
	ValidId          string             `db:"valid_id"`
	DocumentStatus   string             `db:"document_status"`
	Terminals        []structs.Terminal `db:"-"`
}

// PaginatedMerchantsResult structures the response for paginated merchant lists.
type PaginatedMerchantsResult struct {
	Merchants  []structs.Merchant `json:"merchants"`
	TotalItems int                `json:"totalItems"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
}

// SimMerchant is a lightweight representation of a merchant, often used in dropdowns or simple lists.
type SimMerchant struct {
	ID   string `db:"merchant_id"`
	Name string `db:"business_name"`
}

// ---------------------------------------------------------------------------
// Terminal Models
// ---------------------------------------------------------------------------

// UnassignedTerminalData holds basic details about a terminal before it's assigned to a merchant.
type UnassignedTerminalData struct {
	TerminalSN string `json:"terminal_sn" db:"terminal_sn"`
	DeviceName string `json:"device_name" db:"device_name"`
	Status     string `json:"status" db:"status"`
}

// PaginatedTerminalsResult structures the response for paginated terminal lists and status counts.
type PaginatedTerminalsResult struct {
	Terminals     []structs.Terminal `json:"terminals"`
	TotalItems    int                `json:"totalItems"`
	Page          int                `json:"page"`
	Limit         int                `json:"limit"`
	ActiveCount   int                `json:"activeCount"`
	InactiveCount int                `json:"inactiveCount"`
}

// TerminalRequest models a merchant's application for a new payment terminal.
type TerminalRequest struct {
	ID           int        `json:"id" db:"id"`
	RequestID    string     `json:"request_id" db:"request_id"`
	MerchantID   string     `json:"merchant_id" db:"merchant_id"`
	TerminalSN   *string    `json:"terminal_sn" db:"terminal_sn"`
	Status       string     `json:"status" db:"status"`
	RequestedAt  time.Time  `json:"requested_at" db:"requested_at"`
	HandledBy    *string    `json:"handled_by" db:"handled_by"`
	HandledAt    *time.Time `json:"handled_at" db:"handled_at"`
	Notes        *string    `json:"notes" db:"notes"`
	BusinessName string     `json:"business_name" db:"business_name"`
	OwnerName    string     `json:"owner_name" db:"owner_name"`
	DeviceName   *string    `json:"device_name" db:"device_name"`
}

// TerminalRequestsResult structures the paginated response for terminal application requests.
type TerminalRequestsResult struct {
	Requests    []TerminalRequest
	TotalItems  int
	CurrentPage int
	TotalPages  int
}

// ---------------------------------------------------------------------------
// Card Models
// ---------------------------------------------------------------------------

// AdminCard represents a smart card entity as viewed from the administrative dashboard.
type AdminCard struct {
	UserID     sql.NullString  `json:"user_id" db:"user_id"`
	CardUID    string          `json:"card_uid" db:"card_uid"`
	CardNumber string          `json:"card_number" db:"card_number"`
	CardType   string          `json:"card_type" db:"card_type"`
	Balance    decimal.Decimal `json:"initial_amount" db:"balance"`
	ExpiryDate string          `json:"expiry_date" db:"expiry_date"`
	Status     string          `json:"status" db:"status"`
	CreatedAt  string          `json:"created_at" db:"created_at"`
}

// AdminCardInventoryStats tracks the distribution of card statuses across the system.
type AdminCardInventoryStats struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
	Blocked  int `json:"blocked"`
	Lost     int `json:"lost"`
}

// CardInventoryResult wraps a list of cards along with system-wide inventory statistics.
type CardInventoryResult struct {
	Stats AdminCardInventoryStats `json:"stats"`
	Cards []AdminCard             `json:"cards"`
}

// ---------------------------------------------------------------------------
// Transaction Models
// ---------------------------------------------------------------------------

// TxnRow represents a detailed financial transaction record for administrative auditing.
type TxnRow struct {
	TransactionID string          `json:"transaction_id" db:"transaction_id"`
	TerminalID    string          `json:"terminal_id" db:"terminal_id"`
	Date          string          `json:"date" db:"date"`
	Time          string          `json:"time" db:"time"`
	Description   string          `json:"description" db:"description"`
	Type          string          `json:"type" db:"type"`
	Amount        decimal.Decimal `json:"amount" db:"amount"`
	Status        string          `json:"status" db:"status"`
	MerchantName  string          `json:"merchant_name" db:"merchant_name"`
	MerchantID    string          `json:"merchant_id" db:"merchant_id"`
	ServiceFee    decimal.Decimal `json:"service_fee" db:"service_fee"`
	PointsEarned  decimal.Decimal `json:"points_earned" db:"points_earned"`
	Sender        string          `json:"sender" db:"sender"`
	Receiver      string          `json:"receiver" db:"receiver"`
	CustomerName  string          `json:"customer_name" db:"customer_name"`
	Source        string          `json:"source" db:"source"`
	CardNumber    string          `json:"card_number" db:"card_number"`
}

// XenditTxnRow models a payout or top-up transaction processed through the Xendit payment gateway.
type XenditTxnRow struct {
	TransactionID    string          `json:"transaction_id" db:"transaction_id"`
	Date             string          `json:"date" db:"date"`
	Time             string          `json:"time" db:"time"`
	Description      string          `json:"description" db:"description"`
	Type             string          `json:"type" db:"type"`
	Amount           decimal.Decimal `json:"amount" db:"amount"`
	Status           string          `json:"status" db:"status"`
	SettlementStatus string          `json:"settlement_status" db:"settlement_status"`
	SettlementTime   string          `json:"settlement_time" db:"settlement_time"`
}

// ---------------------------------------------------------------------------
// View Data
// ---------------------------------------------------------------------------

// AdminPageData holds the necessary template variables to render an admin dashboard page.
type AdminPageData struct {
	Page     string
	Username string
}