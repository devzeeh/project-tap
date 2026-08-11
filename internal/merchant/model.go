package merchant

import "github.com/shopspring/decimal"

// MerchantSummary struct defines the structure of the data returned by the dashboard API,
// including account info and recent transactions.
type MerchantSummary struct {
	Username           string                `json:"username"`
	MerchantID         string                `json:"merchant_id"`
	AccountRole        string                `json:"role"`
	AccountStatus      string                `json:"account_status"`
	TotalTransactions  int                   `json:"total_transactions"`
	GrossRevenue       decimal.Decimal       `json:"gross_revenue"`
	TotalRefunds       decimal.Decimal       `json:"total_refunds"`
	NetRevenue         decimal.Decimal       `json:"net_revenue"`
	TotalServiceFee    decimal.Decimal       `json:"total_service_fee"`
	TotalIncome        decimal.Decimal       `json:"total_income"`
	AvailableBalance   decimal.Decimal       `json:"available_balance"`
	MonthlyNetIncome   decimal.Decimal       `json:"monthly_net_income"`
	SettlementBank     *string               `json:"settlement_bank"`
	SettlementAccount  *string               `json:"settlement_account_number"`
	SettlementName     *string               `json:"settlement_account_name"`
	RecentTransactions []MerchantTransaction `json:"recent_transactions"`
}

// MerchantDetails represents the internal database structure for a merchant's detailed profile.
type MerchantDetails struct {
	merchantID        string `db:"merchant_id"`
	accountStatus     string `db:"status"`
	businessName      string `db:"business_name"`
	businessType      string `db:"business_type"`
	businessStructure string `db:"business_structure"`
	businessEmail     string `db:"business_email"`
	businessPhone     string `db:"business_phone"`
	businessAddress   string `db:"business_address"`
	city              string `db:"city"`
	postalCode        string `db:"postal_code"`
	accName           string `db:"settlement_account_name"`
	bankName          string `db:"settlement_bank_name"`
	accNumber         string `db:"settlement_account_number"`
	businessDoc       string `db:"business_document,bir_document"`
	birDoc            string `db:"bir_document"`
	validID           string `db:"other_document"` // valid_id
	bankDoc           string `db:"bank_document"`  //
	docStatus         string `db:"document_status"`
	docMessage        string `db:"message"`
	createdAtStr      string `db:"created_at"`
}

// MerchantTransaction represents a single financial transaction associated with the merchant.
type MerchantTransaction struct {
	TransactionID     string          `json:"transaction_id" db:"transaction_id"`
	CardNumber        string          `json:"card_number" db:"card_number"`
	MerchantID        *string         `json:"merchant_id" db:"merchant_id"`
	TerminalID        *string         `json:"terminal_id" db:"terminal_id"`
	TransactionType   string          `json:"transaction_type" db:"transaction_type"`
	Amount            decimal.Decimal `json:"amount" db:"transaction_amount"`
	Points            decimal.Decimal `json:"points" db:"points_earned"`
	ServiceFee        decimal.Decimal `json:"service_fee" db:"service_fee"`
	NetMerchantPayout decimal.Decimal `json:"net_merchant_payout" db:"net_merchant_payout"`
	ProcessedBy       *string         `json:"processed_by" db:"processed_by"`
	Description       *string         `json:"description" db:"description"`
	Date              string          `json:"created_at" db:"created_at"`
	Status            string          `json:"status" db:"status"`
}

// IncomeHistory represents a single income or payout transaction for the merchant.
type IncomeHistory struct {
	Date            string          `json:"date" db:"created_at"`
	Description     *string         `json:"description" db:"description"`
	TransactionID   string          `json:"transaction_id" db:"transaction_id"`
	CardNumber      string          `json:"card_number" db:"card_number"`
	TransactionType string          `json:"transaction_type" db:"transaction_type"`
	Amount          decimal.Decimal `json:"amount" db:"amount"`
	NetIncome       decimal.Decimal `json:"net_income" db:"net_merchant_payout"`
	ServiceFee      decimal.Decimal `json:"service_fee" db:"service_fee"`
	ProcessedBy     *string         `json:"processed_by" db:"processed_by"`
	TerminalID      *string         `json:"terminal_id" db:"terminal_id"`
}

// IncomeStat represents the merchant's income statistics
type IncomeStat struct {
	NetRevenue       decimal.Decimal `json:"net_revenue"`        // What the merchant gets after platform fee
	GrossRevenue     decimal.Decimal `json:"gross_revenue"`      // SUM(amount) payments
	PlatformFee      decimal.Decimal `json:"platform_fee"`       // SUM(service_fee) payments
	TotalRefunds     decimal.Decimal `json:"total_refunds"`      // SUM(amount) refunds all-time
	MonthlyNetIncome decimal.Decimal `json:"monthly_net_income"` // Net income for the current month
	MonthlyRefunds   decimal.Decimal `json:"monthly_refunds"`    // Refunds for the current month
	TotalWithdrawn   decimal.Decimal `json:"total_withdrawn"`    // Total amount withdrawn by the merchant
	AvailableBalance decimal.Decimal `json:"available_balance"`  // What the merchant can withdraw (NetRevenue - TotalWithdrawn)
	MonthlyWithdrawn decimal.Decimal `json:"monthly_withdrawn"`  // Withdrawals for the current month (if needed in the future)
}

// IncomeResponse encapsulates the merchant's income statistics and transaction history.
type IncomeResponse struct {
	Stats   IncomeStat      `json:"stats"`
	History []IncomeHistory `json:"history"`
}

// BankDetails holds the internal settlement bank account information.
type BankDetails struct {
	merchantID            string
	settlementBank        *string
	settlementAccountName *string
	settlementAccount     *string
}

// WithdrawRequest represents the JSON payload for a merchant withdrawal request.
type WithdrawRequest struct {
	Amount decimal.Decimal `json:"amount"`
}

// BusinessDetails holds the core profile information of the merchant's business.
type BusinessDetails struct {
	BusinessName    string `json:"business_name"`
	BusinessType    string `json:"business_type"`
	BusinessEmail   string `json:"business_email"`
	BusinessPhone   string `json:"business_phone"`
	BusinessAddress string `json:"business_address"`
	City            string `json:"city"`
	PostalCode      string `json:"postal_code"`
}

// BusinessDocument represents a document uploaded by the merchant for verification purposes.
type BusinessDocument struct {
	DocumentType      string `json:"document_type"`
	Status            string `json:"document_status"`
	Message           string `json:"message"`
	BusinessStructure string `json:"business_structure"`
	DocumentURL       string `json:"document_url"`
}

// BusinessBankDetails represents the merchant's external settlement bank account information.
type BusinessBankDetails struct {
	AccountHolderName string `json:"account_holder_name"`
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
}

// AccountSummary provides a comprehensive overview of the merchant's account status and profile.
type AccountSummary struct {
	MerchantID          string              `json:"merchant_id"`
	AccountStatus       string              `json:"account_status"`
	DocumentStatus      string              `json:"document_status"`
	AccountMessage      string              `json:"account_message"`
	MemberSince         string              `json:"member_since"`
	BusinessDetails     BusinessDetails     `json:"business_details"`
	BusinessBankDetails BusinessBankDetails `json:"business_bank_details"`
	BusinessDocuments   []BusinessDocument  `json:"business_document"`
}

// MerchantPageData holds the contextual data passed to the merchant HTML templates.
type MerchantPageData struct {
	Page     string
	Username string
}
