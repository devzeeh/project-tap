package auth

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"project-tap/internal/pkg/database"
)

type Repository struct {
	store database.Store
}

func NewRepository(store database.Store) *Repository {
	return &Repository{store: store}
}

// Finds a user by username or email or phone number
// Used by the handler to find a user by username or email or phone number
// It does not use user id because the user id is not known at this point
func (r *Repository) FindUserByUsername(ctx context.Context, username string) (User, error) {
	const query = `SELECT id, username, password_hash, role
		FROM users
		WHERE email = ? OR username = ? OR phone_number = ?`

	var u User
	err := r.store.QueryRowContext(ctx, query, username, username, username).
		Scan(&u.UserID, &u.Email, &u.PasswordHash, &u.Role)
	if err != nil {
		log.Printf("error finding user %s: %v", username, err)
		return User{}, fmt.Errorf("username %s not found: %w", username, err)
	}

	return u, nil
}

// Checks if a user with the given user ID exists
// Used by the handler to check if the user ID exists
func (r *Repository) IsUserIDExist(ctx context.Context, userID int64) (bool, error) {
	var existing int64
	const query = `SELECT user_id FROM users WHERE user_id = ?`
	err := r.store.QueryRowContext(ctx, query, userID).Scan(&existing)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		log.Printf("error checking if user %d exists: %v", userID, err)
		return false, fmt.Errorf("error checking if user %d exists: %w", userID, err)
	}
	return true, nil
}

// Checks if a user with the given phone number exists
// Used by the handler to check if the phone number exists
func (r *Repository) IsPhoneExist(ctx context.Context, phoneNumber string) (bool, error) {
	var existing string
	const query = `SELECT phone_number FROM users WHERE phone_number = ?`
	err := r.store.QueryRowContext(ctx, query, phoneNumber).Scan(&existing)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		log.Printf("error checking if phone %s exists: %v", phoneNumber, err)
		return false, fmt.Errorf("error checking if phone %s exists: %w", phoneNumber, err)
	}
	return true, nil
}

// Gets the initial balance of a user
// Used by the handler to get the initial balance of a user
func (r *Repository) GetInitialBalance(ctx context.Context, cardNumber string) (float64, error) {
	var balance float64
	const query = `SELECT balance FROM cards WHERE card_number = ?`
	err := r.store.QueryRowContext(ctx, query, cardNumber).Scan(&balance)
	if err != nil {
		log.Printf("GetInitialBalance error for card %s: %v", cardNumber, err)
		return 0, fmt.Errorf("error getting initial balance for card %s: %w", cardNumber, err)
	}
	return balance, nil
}

// Gets the status of a card
// Used by the handler to get the status of a card
func (r *Repository) GetCardStatus(ctx context.Context, cardNumber string) (string, error) {
	var status string
	const query = `SELECT status FROM cards WHERE card_number = ?`
	err := r.store.QueryRowContext(ctx, query, cardNumber).Scan(&status)
	if err != nil {
		log.Printf("GetCardStatus error for card %s: %v", cardNumber, err)
		return "", fmt.Errorf("error getting card status for card %s: %w", cardNumber, err)
	}
	return status, nil
}

// Updates the password of a user
// Used by the handler to update the password of a user
func (r *Repository) UpdatePassword(ctx context.Context, email, hashedPassword string) error {
	const query = `UPDATE users SET password_hash = ? WHERE email = ?`
	_, err := r.store.ExecContext(ctx, query, hashedPassword, email)
	if err != nil {
		log.Printf("failed to update password: %v", err)
		return err
	}
	return nil
}

// Finds a user by email
// Used by the handler to find a user by email
func (r *Repository) FindUserByEmail(ctx context.Context, email string) (string, string, error) {
	var name, userID string
	const query = `SELECT name, user_id FROM users WHERE email = ?`
	err := r.store.QueryRowContext(ctx, query, email).Scan(&name, &userID)
	if err != nil {
		log.Printf("error finding user %s: %v", email, err)
		return "", "", fmt.Errorf("username %s not found: %w", email, err)
	}
	return name, userID, nil
}

// FindNameByEmail returns only the display name for OTP emails.
func (r *Repository) FindNameByEmail(ctx context.Context, email string) string {
	var name string
	const query = `SELECT name FROM users WHERE email = ?`
	err := r.store.QueryRowContext(ctx, query, email).Scan(&name)
	if err != nil {
		log.Printf("error finding user %s: %v", email, err)
		return "there" // safe fallback for email greeting
	}
	return name
}

// InsertActivityLog writes a row to user_activity_logs (best-effort; errors
// are logged but do not fail the parent request).
func (r *Repository) InsertActivityLog(ctx context.Context, userID, activityType, channel, status, description string) {
	const query = `
		INSERT INTO user_activity_logs (user_id, activity_type, channel, status, description)
		VALUES (?, ?, ?, ?, ?)`
	_, err := r.store.ExecContext(ctx, query, userID, activityType, channel, status, description)
	if err != nil {
		log.Printf("activity log insert failed [%s/%s]: %v", userID, activityType, err)
	}
}
