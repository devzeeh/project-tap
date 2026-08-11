package auth

import "time"

// User is the full users table row used across auth flows.
type User struct {
	UserID       string  `db:"user_id"`
	Username     string  `db:"username"`
	Name         string  `db:"name"`
	Email        string  `db:"email"`
	Phone        string  `db:"phone_number"`
	CardNumber   string  `db:"card_number"`
	PasswordHash string  `db:"password_hash"`
	Role         string  `db:"role"`
	Balance      float64 `db:"balance"`
	Status       string  `db:"status"`
	RegionID     string  `db:"region_id"`
	CreatedAt    string  `db:"created_at"`
}

// OTPData holds a one-time password and its expiry for in-memory OTP tracking.
// TODO: move to Redis for multi-instance deployments.
type OTPData struct {
	OTP    string
	Expiry time.Time
}