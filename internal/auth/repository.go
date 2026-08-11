package auth

import (
	"database/sql"
	"fmt"
	"log"

	"project-tap/internal/pkg/database"
)

// Repository handles every database operation needed by the auth package.
type Repository struct {
	store database.Store
}

// NewRepository creates a new instance of Repository connected to the given store.
func NewRepository(store database.Store) *Repository {
	return &Repository{store: store}
}

// FindUserByIdentifier looks up a user by email, username, or phone number.
func (r *Repository) FindUserByIdentifier(identifier string) (User, error) {
	const stmt = `SELECT id, username, password_hash, role
	              FROM users
	              WHERE email = ? OR username = ? OR phone_number = ?`
	var u User
	err := r.store.QueryRow(stmt, identifier, identifier, identifier).
		Scan(&u.UserID, &u.Username, &u.PasswordHash, &u.Role)
	if err != nil {
		return User{}, fmt.Errorf("find user by identifier: %w", err)
	}
	return u, nil
}

// IsUserIDExist returns true if the given numeric user ID is already taken.
func (r *Repository) IsUserIDExist(userID int64) (bool, error) {
	var tmp int64
	err := r.store.QueryRow("SELECT user_id FROM users WHERE user_id = ?", userID).Scan(&tmp)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// IsPhoneExist returns true if the phone number is already registered.
func (r *Repository) IsPhoneExist(phone string) (bool, error) {
	var existing string
	err := r.store.QueryRow("SELECT phone_number FROM users WHERE phone_number = ?", phone).Scan(&existing)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		log.Printf("phone check error: %v", err)
		return false, err
	}
	return true, nil
}

// GetInitialBalance returns the current balance on a card (used during signup
// to carry over any pre-loaded balance).
func (r *Repository) GetInitialBalance(cardNumber string) (float64, error) {
	var balance float64
	err := r.store.QueryRow("SELECT balance FROM cards WHERE card_number = ?", cardNumber).Scan(&balance)
	if err != nil {
		log.Printf("GetInitialBalance error for card %s: %v", cardNumber, err)
		return 0, err
	}
	return balance, nil
}

// GetCardStatus returns the status field for a card number.
func (r *Repository) GetCardStatus(cardNumber string) (string, error) {
	var status string
	err := r.store.QueryRow("SELECT status FROM cards WHERE card_number = ?", cardNumber).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("get card status: %w", err)
	}
	return status, nil
}

// UpdatePassword sets a new bcrypt hash for the user with the given email.
func (r *Repository) UpdatePassword(email, hashedPassword string) error {
	_, err := r.store.Exec("UPDATE users SET password_hash = ? WHERE email = ?", hashedPassword, email)
	if err != nil {
		log.Printf("failed to update password: %v", err)
		return err
	}
	return nil
}

// FindUserByEmail returns the name and user_id for a given email address.
func (r *Repository) FindUserByEmail(email string) (name, userID string, err error) {
	err = r.store.QueryRow("SELECT name, user_id FROM users WHERE email = ?", email).Scan(&name, &userID)
	return
}

// FindNameByEmail returns only the display name for OTP emails.
func (r *Repository) FindNameByEmail(email string) string {
	var name string
	if err := r.store.QueryRow("SELECT name FROM users WHERE email = ?", email).Scan(&name); err != nil {
		return "there" // safe fallback for email greeting
	}
	return name
}

// InsertActivityLog writes a row to user_activity_logs (best-effort; errors
// are logged but do not fail the parent request).
func (r *Repository) InsertActivityLog(userID, activityType, channel, status, description string) {
	_, err := r.store.Exec(`
		INSERT INTO user_activity_logs (user_id, activity_type, channel, status, description)
		VALUES (?, ?, ?, ?, ?)`,
		userID, activityType, channel, status, description,
	)
	if err != nil {
		log.Printf("activity log insert failed [%s/%s]: %v", userID, activityType, err)
	}
}