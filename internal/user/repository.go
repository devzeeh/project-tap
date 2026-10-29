package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"project-tap/internal/pkg/database"

	"github.com/shopspring/decimal"
)

// Repository handles simple, reusable DB queries for the merchant package.
type Repository struct {
	store database.Store
}

// NewRepository creates a new instance of Repository using the provided database store.
func NewRepository(store database.Store) *Repository {
	return &Repository{store: store}
}

type userCardRow struct {
	id            int
	username      string
	fullName      string
	email         string
	pendingEmail  string
	phone         string
	userType      string
	balance       float64
	loyaltyPoints decimal.Decimal
	cardNumber    string
	expiryDate    string
	cardStatus    string
	userStatus    string
}

type recentTransaction struct {
	TransactionID string          `json:"transaction_id"`
	TerminalID    string          `json:"terminal_id"`
	Date          string          `json:"date" db:"date"`
	Time          string          `json:"time"`
	Description   string          `json:"description" db:"description"`
	Type          string          `json:"type" db:"transaction_type"`
	Amount        decimal.Decimal `json:"amount" db:"transaction_amount"`
	Status        string          `json:"status" db:"status"`
	MerchantName  string          `json:"merchant_name"`
	MerchantID    string          `json:"merchant_id"`
	ServiceFee    float64         `json:"service_fee"`
	PointsEarned  decimal.Decimal `json:"points_earned"`
}

type userTransactionRow struct {
	transactionID string
	terminalID    string
	createdAt     string
	txnType       string
	amount        float64
	status        string
	description   string
	businessName  string
	merchantID    string
	pointsEarned  decimal.Decimal
	cardNumber    string
}

type UserCard struct {
	UserID     string
	Balance    decimal.Decimal
	CardNumber string
}

type CardPayerInfo struct {
	CardNumber string
	Email      string
}

type PendingTopUp struct {
	CardNumber     string
	Amount         float64
	ConvenienceFee float64
	Status         string
}

func (r *Repository) GetRecentTransaction(ctx context.Context, username string) ([]Transaction, error) {
	rows, err := r.store.QueryContext(ctx,
		`(SELECT 
        t.transaction_id, 
        COALESCE(t.description, '') AS description, 
        t.created_at, 
        COALESCE(t.transaction_type, '') AS transaction_type, 
        t.amount, 
        COALESCE(t.terminal_id, '') AS terminal_id, 
        COALESCE(t.status, '') AS status, 
        COALESCE(m.business_name, '') AS business_name, 
        COALESCE(m.merchant_id, '') AS merchant_id, 
        COALESCE(t.points_earned, 0) AS points_earned
    FROM transactions t 
    JOIN users u ON t.user_id = u.user_id
    LEFT JOIN merchants m ON t.merchant_id = m.merchant_id
    WHERE u.username = ?)
    UNION ALL
    (SELECT 
        CONCAT('LOG-', ual.id) AS transaction_id, 
        ual.description, 
        ual.created_at, 
        ual.activity_type AS transaction_type, 
        0.00 AS amount, 
        '' AS terminal_id, 
        ual.status, 
        '' AS business_name, 
        '' AS merchant_id, 
        0 AS points_earned
    FROM user_activity_logs ual
    JOIN users u ON ual.user_id = u.user_id
    WHERE u.username = ?)
    ORDER BY created_at DESC LIMIT 5`, username, username)

	if err != nil {
		return nil, fmt.Errorf("get recent user transactions (username %q): %w", username, err)
	}
	defer rows.Close()

	var result []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(
			&t.TransactionID, &t.Description, &t.CreatedAt, &t.Type, &t.Amount,
			&t.TerminalID, &t.Status, &t.BusinessName, &t.MerchantID, &t.PointsEarned,
		); err != nil {
			return nil, fmt.Errorf("scan recent user transaction row (username %q): %w", username, err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent user transactions (username %q): %w", username, err)
	}
	return result, nil
}

// Dashboard

func (r *Repository) GetUserByusername(ctx context.Context, username string) (*userCardRow, error) {
	var row userCardRow

	err := r.store.QueryRowContext(ctx,
		`SELECT 
			u.id,
			u.username,
			u.name,
			u.email,
			COALESCE(u.pending_email, ''),
			COALESCE(u.phone_number, ''),
			u.role,
			COALESCE(c.balance, 0),
			COALESCE(c.loyalty_points, 0),
			COALESCE(c.card_number, ''),
			COALESCE(c.expiry_date, ''),
			COALESCE(c.status, ''),
			COALESCE(u.status, '')
		FROM users u
		LEFT JOIN cards c 
			ON u.user_id = c.user_id
		WHERE u.username = ?`, username).Scan(
		&row.id, &row.username, &row.fullName, &row.email, &row.pendingEmail,
		&row.phone, &row.userType, &row.balance, &row.loyaltyPoints, &row.cardNumber,
		&row.expiryDate, &row.cardStatus, &row.userStatus)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user card by username (username %q): %w", username, err)
	}

	return &row, nil
}

// Customer profile

func (r *Repository) GetPasswordHash(ctx context.Context, username string) (string, error) {
	var hash string
	err := r.store.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE username = ?", username).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("get password hash (username %q): %w", username, err)
	}
	return hash, nil
}

func (r *Repository) GetEmailAndName(ctx context.Context, username string) (email, name string, err error) {
	err = r.store.QueryRowContext(ctx, "SELECT email, name FROM users WHERE username = ?", username).Scan(&email, &name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrUserNotFound
		}
		return "", "", fmt.Errorf("get email and name (username %q): %w", username, err)
	}
	return email, name, nil
}

// UpdateProfileFields runs a dynamic UPDATE users SET ... using the given
// column=value pairs. Column names always come from the service's fixed
// internal mapping, never directly from request input.
func (r *Repository) UpdateProfileFields(ctx context.Context, username string, fields []string, args []any) error {
	if len(fields) == 0 {
		return nil
	}
	query := "UPDATE users SET " + strings.Join(fields, ", ") + " WHERE username = ?"
	args = append(args, username)

	if _, err := r.store.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update profile fields (username %q): %w", username, err)
	}
	return nil
}

func (r *Repository) UpdatePasswordHash(ctx context.Context, username, hash string) error {
	if _, err := r.store.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE username = ?", hash, username); err != nil {
		return fmt.Errorf("update password hash (username %q): %w", username, err)
	}
	return nil
}

// Cards

func (r *Repository) GetUserID(ctx context.Context, username string) (string, error) {
	var userID string
	err := r.store.QueryRowContext(ctx, `SELECT user_id FROM users WHERE username = ?`, username).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("get user id (username %q): %w", username, err)
	}
	return userID, nil
}

func (r *Repository) UpdateCardStatus(ctx context.Context, status, userID string) error {
	_, err := r.store.ExecContext(ctx, `UPDATE cards SET status = ? WHERE user_id = ?`, status, userID)
	if err != nil {
		return fmt.Errorf("update card status (user %q): %w", userID, err)
	}
	return nil
}

func (r *Repository) GetCardBalance(ctx context.Context, userID string) (*UserCard, error) {
	var c UserCard
	c.UserID = userID
	err := r.store.QueryRowContext(ctx, "SELECT balance, card_number FROM cards WHERE user_id = ?", userID).
		Scan(&c.Balance, &c.CardNumber)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCardNotFound
		}
		return nil, fmt.Errorf("get card balance (user %q): %w", userID, err)
	}
	return &c, nil
}

// ReplaceCard atomically deducts the replacement fee, locks the card, and
// records the fee transaction. All three succeed or none do.
func (r *Repository) ReplaceCard(ctx context.Context, userID, cardNumber, txnID string, replacementFee float64) error {
	tx, err := r.store.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace card (user %q): begin tx: %w", userID, err)
	}
	defer tx.Rollback() // no-op if Commit succeeds

	if _, err := tx.ExecContext(ctx,
		"UPDATE cards SET balance = balance - ?, status = 'blocked' WHERE user_id = ?",
		replacementFee, userID,
	); err != nil {
		return fmt.Errorf("replace card (user %q): update card: %w", userID, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (transaction_id, card_number, user_id, transaction_type, amount, status, description)
		VALUES (?, ?, ?, 'payment', ?, 'completed', 'Card Replacement Fee')`,
		txnID, cardNumber, userID, replacementFee,
	); err != nil {
		return fmt.Errorf("replace card (user %q): insert transaction: %w", userID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("replace card (user %q): commit: %w", userID, err)
	}

	return nil
}

func (r *Repository) GetCardAndEmailByUsername(ctx context.Context, username string) (*CardPayerInfo, error) {
	var info CardPayerInfo
	err := r.store.QueryRowContext(ctx, `
		SELECT c.card_number, u.email 
		FROM cards c 
		JOIN users u ON c.user_id = u.user_id 
		WHERE u.username = ? LIMIT 1`, username).Scan(&info.CardNumber, &info.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCardNotFound
		}
		return nil, fmt.Errorf("get card and email (username %q): %w", username, err)
	}
	return &info, nil
}

func (r *Repository) InsertPendingTopUp(ctx context.Context, topupID, cardNumber string, amount, fee decimal.Decimal) error {
	_, err := r.store.ExecContext(ctx, `
		INSERT INTO top_ups (topup_id, card_number, amount, convenience_fee, gateway_cost, payment_method, handled_by, external_id, status) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		topupID, cardNumber, amount, fee, 0.0, "xendit", "payment gateway", topupID, "pending")
	if err != nil {
		return fmt.Errorf("insert pending topup (topup %q): %w", topupID, err)
	}
	return nil
}

func (r *Repository) GetUserIDByCardNumber(ctx context.Context, cardNumber string) (string, error) {
	var userID string
	err := r.store.QueryRowContext(ctx, "SELECT user_id FROM cards WHERE card_number = ?", cardNumber).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrCardNotFound
		}
		return "", fmt.Errorf("get user id by card number (card %q): %w", cardNumber, err)
	}
	return userID, nil
}

// CompleteTopUp atomically credits the card balance, records the top-up
// ledger entry, and records the spending-ledger transaction. All three
// succeed together or none do.
func (r *Repository) CompleteTopUp(ctx context.Context, rec TopUpRecord, topupID, transactionID, userID string) error {
	tx, err := r.store.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("complete topup (card %q): begin tx: %w", rec.CardNumber, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE cards SET balance = balance + ? WHERE card_number = ?`,
		rec.Amount, rec.CardNumber,
	); err != nil {
		return fmt.Errorf("complete topup (card %q): update balance: %w", rec.CardNumber, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO top_ups (topup_id, card_number, amount, convenience_fee, gateway_cost, payment_method) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		topupID, rec.CardNumber, rec.Amount, rec.ConvenienceFee, rec.GatewayCost, rec.PaymentMethod,
	); err != nil {
		return fmt.Errorf("complete topup (card %q): insert topup ledger: %w", rec.CardNumber, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (transaction_id, card_number, user_id, merchant_id, terminal_id, transaction_type, amount, service_fee, processed_by, status, description) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		transactionID, rec.CardNumber, userID, sql.NullString{}, sql.NullString{}, "topup",
		rec.Amount, rec.ConvenienceFee, sql.NullString{}, "completed", "Topup completed",
	); err != nil {
		return fmt.Errorf("complete topup (card %q): insert transaction: %w", rec.CardNumber, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("complete topup (card %q): commit: %w", rec.CardNumber, err)
	}
	return nil
}

func (r *Repository) GetUserTransactions(ctx context.Context, username string) ([]userTransactionRow, error) {
	rows, err := r.store.QueryContext(ctx, `
		(SELECT 
			t.transaction_id, 
			COALESCE(t.terminal_id, '') AS terminal_id, 
			t.created_at, 
			COALESCE(t.transaction_type, '') AS transaction_type, 
			t.amount, 
			COALESCE(t.status, '') AS status,
			COALESCE(t.description, '') AS description,
			COALESCE(m.business_name, '') AS business_name,
			COALESCE(m.merchant_id, '') AS merchant_id,
			COALESCE(t.points_earned, 0) AS points_earned,
			COALESCE(c.card_number, '') AS card_number
		FROM transactions t
		LEFT JOIN cards c ON t.card_number = c.card_number 
		JOIN users u ON t.user_id = u.user_id
		LEFT JOIN merchants m ON t.merchant_id = m.merchant_id
		WHERE u.username = ?)
		UNION ALL
		(SELECT 
			CONCAT('LOG-', ual.id) AS transaction_id, 
			'' AS terminal_id, 
			ual.created_at, 
			ual.activity_type AS transaction_type, 
			0.00 AS amount, 
			ual.status,
			ual.description,
			'' AS business_name,
			'' AS merchant_id,
			0 AS points_earned,
			'' AS card_number
		FROM user_activity_logs ual
		JOIN users u ON ual.user_id = u.user_id
		WHERE u.username = ?)
		ORDER BY created_at DESC
	`, username, username)
	if err != nil {
		return nil, fmt.Errorf("get user transactions (username %q): %w", username, err)
	}
	defer rows.Close()

	var results []userTransactionRow
	for rows.Next() {
		var t userTransactionRow
		if err := rows.Scan(
			&t.transactionID, &t.terminalID, &t.createdAt, &t.txnType, &t.amount,
			&t.status, &t.description, &t.businessName, &t.merchantID,
			&t.pointsEarned, &t.cardNumber,
		); err != nil {
			return nil, fmt.Errorf("scan user transaction row (username %q): %w", username, err)
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user transactions (username %q): %w", username, err)
	}

	return results, nil
}

func (r *Repository) GetPendingTopUp(ctx context.Context, tx *sql.Tx, topupID string) (*PendingTopUp, error) {
	var t PendingTopUp
	err := tx.QueryRowContext(ctx, `SELECT card_number, amount, convenience_fee, status FROM top_ups WHERE topup_id = ?`, topupID).
		Scan(&t.CardNumber, &t.Amount, &t.ConvenienceFee, &t.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTopUpNotFound
		}
		return nil, fmt.Errorf("get pending topup (topup %q): %w", topupID, err)
	}
	return &t, nil
}

// CompletePendingTopUp credits the card balance, marks the top-up ledger
// completed, and upserts the matching transaction row — all atomically.
// Returns ErrTopUpAlreadyCompleted (non-fatal, safe to ignore) if the
// webhook was already processed, so callers can ack Xendit without retrying.
func (r *Repository) CompletePendingTopUp(ctx context.Context, topupID string) error {
	tx, err := r.store.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("complete pending topup (topup %q): begin tx: %w", topupID, err)
	}
	defer tx.Rollback()

	topUp, err := r.GetPendingTopUp(ctx, tx, topupID)
	if err != nil {
		if errors.Is(err, ErrTopUpNotFound) {
			// Not found: ignore rather than fail, so Xendit doesn't retry forever.
			return nil
		}
		return err
	}

	if topUp.Status == "completed" {
		return ErrTopUpAlreadyCompleted
	}

	if _, err := tx.ExecContext(ctx, `UPDATE cards SET balance = balance + ? WHERE card_number = ?`, topUp.Amount, topUp.CardNumber); err != nil {
		return fmt.Errorf("complete pending topup (topup %q): update balance: %w", topupID, err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE top_ups SET status = 'completed' WHERE topup_id = ?`, topupID); err != nil {
		return fmt.Errorf("complete pending topup (topup %q): update topup status: %w", topupID, err)
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE transactions SET status = 'completed', description = 'Successful topup via Xendit' 
		WHERE card_number = ? AND transaction_type = 'topup' AND status = 'pending' AND amount = ? 
		ORDER BY created_at DESC LIMIT 1`, topUp.CardNumber, topUp.Amount)
	if err != nil {
		return fmt.Errorf("complete pending topup (topup %q): update transaction: %w", topupID, err)
	}

	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		var userID string
		if err := tx.QueryRowContext(ctx, "SELECT user_id FROM cards WHERE card_number = ?", topUp.CardNumber).Scan(&userID); err != nil {
			return fmt.Errorf("complete pending topup (topup %q): lookup user id: %w", topupID, err)
		}

		transactionID := fmt.Sprintf("TX-%d", time.Now().UnixNano())
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transactions (transaction_id, card_number, user_id, merchant_id, terminal_id, transaction_type, amount, service_fee, processed_by, description, status) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			transactionID, topUp.CardNumber, userID, "xendit", "xendit", "topup", topUp.Amount, topUp.ConvenienceFee, "xendit", "Successful topup via Xendit", "completed",
		); err != nil {
			return fmt.Errorf("complete pending topup (topup %q): insert transaction: %w", topupID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("complete pending topup (topup %q): commit: %w", topupID, err)
	}

	log.Printf("Successfully loaded ₱%.2f onto card %s via Xendit", topUp.Amount, topUp.CardNumber)
	return nil
}

func (r *Repository) UpdateTopUpStatus(ctx context.Context, topupID, status string) error {
	if _, err := r.store.ExecContext(ctx, `UPDATE top_ups SET status = ? WHERE topup_id = ?`, status, topupID); err != nil {
		return fmt.Errorf("update topup status (topup %q): %w", topupID, err)
	}
	return nil
}

func (r *Repository) DeletePendingTopUpTransaction(ctx context.Context, topupID string) error {
	var cardNumber string
	var amount float64
	err := r.store.QueryRowContext(ctx, `SELECT card_number, amount FROM top_ups WHERE topup_id = ?`, topupID).Scan(&cardNumber, &amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // nothing to clean up
		}
		return fmt.Errorf("delete pending topup transaction (topup %q): lookup: %w", topupID, err)
	}

	if _, err := r.store.ExecContext(ctx, `
		DELETE FROM transactions WHERE card_number = ? AND transaction_type = 'topup' AND status = 'pending' AND amount = ? 
		ORDER BY created_at DESC LIMIT 1`, cardNumber, amount); err != nil {
		return fmt.Errorf("delete pending topup transaction (topup %q): %w", topupID, err)
	}
	return nil
}

func (r *Repository) GetPendingEmailByToken(ctx context.Context, token string) (username, pendingEmail string, err error) {
	err = r.store.QueryRowContext(ctx, "SELECT username, pending_email FROM users WHERE email_verification_token = ?", token).
		Scan(&username, &pendingEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrInvalidToken
		}
		return "", "", fmt.Errorf("get pending email by token: %w", err)
	}
	return username, pendingEmail, nil
}

func (r *Repository) ConfirmEmailChange(ctx context.Context, username, newEmail string) error {
	_, err := r.store.ExecContext(ctx,
		"UPDATE users SET email = ?, pending_email = NULL, email_verification_token = NULL WHERE username = ?",
		newEmail, username)
	if err != nil {
		return fmt.Errorf("confirm email change (username %q): %w", username, err)
	}
	return nil
}

func (r *Repository) GetUsernameByCardNumber(ctx context.Context, cardNumber string) (string, error) {
	var username string
	err := r.store.QueryRowContext(ctx, `
		SELECT u.username 
		FROM users u 
		JOIN cards c ON u.user_id = c.user_id 
		WHERE c.card_number = ? LIMIT 1
	`, cardNumber).Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}

func (r *Repository) GetUsernameByTopUpID(ctx context.Context, topupID string) (string, error) {
	var username string
	err := r.store.QueryRowContext(ctx, `
		SELECT u.username 
		FROM users u 
		JOIN cards c ON u.user_id = c.user_id 
		JOIN top_ups tu ON c.card_number = tu.card_number 
		WHERE tu.topup_id = ? LIMIT 1
	`, topupID).Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}
