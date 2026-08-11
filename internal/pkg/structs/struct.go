package structure

import "github.com/shopspring/decimal"

// CardData struct represents the data required to create a new card
type CardData struct {
	CardUID    string          `json:"card_uid" db:"card_uid" validate:"required"`
	CardNumber string          `json:"cardNumber" db:"card_number"`
	CardHolder string          `json:"cardHolder" db:"user_id"`
	CardType   string          `json:"cardType" db:"card_type"`
	Balance    decimal.Decimal `json:"initial_amount" db:"balance" validate:"required"`
}

// List of all merchants
// Merchant represents a single business tenant to be displayed in the data table
type Merchant struct {
	MerchantID   string     `json:"merchant_id"`
	BusinessName string     `json:"business_name"`
	BusinessType string     `json:"business_type"`
	OwnerName    string     `json:"owner_name"`
	Email        string     `json:"business_email"`
	Phone        string     `json:"business_phone"`
	Status       string     `json:"status"`
	CreatedAt    string     `json:"created_at"`
	Terminals    []Terminal `json:"terminals,omitempty"`
}

// AdminDashboardData struct represents the data to be displayed on the admin dashboard
type AdminDashboardData struct {
	GrossRevenue       decimal.Decimal `json:"grossRevenue"`
	NetRevenue         decimal.Decimal `json:"netRevenue"`
	TotalUsers         int             `json:"totalUsers"`
	TotalCards         int             `json:"totalCards"`
	TotalMerchants     int             `json:"totalMerchants"`
	PendingMerchants   int             `json:"pendingMerchants"`
	SuspendedMerchants int             `json:"suspendedMerchants"`
	RejectedMerchants  int             `json:"rejectedMerchants"`
	TotalTerminals     int             `json:"totalTerminals"`
	ActiveTerminals    int             `json:"activeTerminals"`
	InactiveTerminals  int             `json:"inactiveTerminals"`
	Merchants          []Merchant      `json:"merchants"`
}

// Terminal represents a hardware device in the terminal registry
type Terminal struct {
	TerminalID      string `json:"terminal_id"`
	TerminalSN      string `json:"terminal_sn"`
	AssignedMerch   string `json:"assigned_merchant"`
	DeviceName      string `json:"device_name"`
	LocationDetails string `json:"location_details"`
	Status          string `json:"status"`
}

// Transaction struct represents a user's transaction for the dashboard view
type Transaction struct {
	Date        string          `db:"date" json:"date"`
	Description string          `db:"description" json:"description"`
	Type        string          `db:"transaction_type" json:"type"`
	Amount      decimal.Decimal `db:"transaction_amount" json:"amount"`
}

// DashboardUser info struct for the user dashboard view
type DashboardUser struct {
	ID                 int             `db:"id" json:"id,omitempty"`
	UserID             string          `db:"user_id" json:"user_id,omitempty"`
	Username           string          `db:"username" json:"username"`
	Name               string          `db:"name" json:"name"`
	Balance            float64         `db:"balance" json:"balance"`
	LoyaltyPoints      decimal.Decimal `db:"loyalty_points" json:"loyalty_points"`
	AccountType        string          `db:"account_type" json:"account_type"`
	RecentTransactions []Transaction   `json:"transactions"` // Add recent transactions to the dashboard response
}
