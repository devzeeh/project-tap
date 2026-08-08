package auth

import "github.com/shopspring/decimal"

// model.go contain the model struct that represent the data entity in the system
// and also contain the request and response struct

// UserRole represents a role in the system
type UserRole string

// Role of the user in the system
// Role Based Access Control (RBAC)
const (
	RoleUser     UserRole = "user"
	RoleMerchant UserRole = "merchant"
	RoleAdmin    UserRole = "admin"
)

// User represents a user in the system
type User struct {
	UserID          string          `json:"user_id"`
	Username        string          `json:"username"`
	FullName        string          `json:"full_name"`
	Email           string          `json:"email"`
	PasswordHash    string          `json:"-"` // ignore password in response
	ConfirmPassword string          `json:"confirm_password,omitempty"`
	CardNumber      string          `json:"card_number"`
	Phone           string          `json:"phone"`
	Balance         decimal.Decimal `json:"balance"`
	Status          string          `json:"status"`
	Role            string          `json:"role"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	IsActive        bool            `json:"is_active"` // True or False
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	ID          string
	Username    string
	Role        string
	RedirectURL string
	Tokens      struct {
		Access  string
		Refresh string
	}
}
