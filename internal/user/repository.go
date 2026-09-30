package user

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"project-tap/internal/pkg/database"

	"github.com/shopspring/decimal"
)

// Repository handles database interactions for the user package.
type Repository struct {
	store database.Store
}

// NewRepository creates a new instance of Repository.
func NewRepository(store database.Store) *Repository {
	return &Repository{store: store}
}

// FindUsernameByUserIDOrID looks up the username corresponding to an auth token user_id, id, or username.
func (r *Repository) FindUsernameByUserIDOrID(ctx context.Context, identifier string) (string, error) {
	var username string
	query := "SELECT username FROM users WHERE user_id = ? OR id = ? OR username = ?"
	err := r.store.QueryRowContext(ctx, query, identifier, identifier, identifier).Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}

// GetDashboardUserData fetches customer profile and card balance data.
func (r *Repository) GetDashboardUserData(ctx context.Context, username string) (DashboardUser, error) {
	stmt := `
		SELECT 
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
		WHERE u.username = ?
	`
	var (
		id            int
		user, fullName, email, pendingEmail, phone, userType, cardNumber, expiryDate, cardStatus, userStatus string
		balance       float64
		loyaltyPoints decimal.Decimal
	)
	err := r.store.QueryRowContext(ctx, stmt, username).Scan(
		&id, &user, &fullName, &email, &pendingEmail, &phone, &userType,
		&balance, &loyaltyPoints, &cardNumber, &expiryDate, &cardStatus, &userStatus,
	)
	if err != nil {
		return DashboardUser{}, err
	}

	initials := ""
	parts := strings.Fields(fullName)
	if len(parts) > 0 {
		initials += string([]rune(parts[0])[0])
		if len(parts) > 1 {
			initials += string([]rune(parts[len(parts)-1])[0])
		}
	}
	if initials == "" {
		initials = "U"
	}
	initials = strings.ToUpper(initials)

	expiryStr := "MM/YY"
	if len(expiryDate) >= 10 {
		tExpiry, errT := time.Parse("2006-01-02", expiryDate[:10])
		if errT == nil {
			expiryStr = tExpiry.Format("01/06")
		}
	}

	return DashboardUser{
		ID:            id,
		UserID:        user,
		Username:      user,
		Name:          fullName,
		Email:         email,
		PendingEmail:  pendingEmail,
		Phone:         phone,
		Initials:      initials,
		Balance:       balance,
		LoyaltyPoints: loyaltyPoints,
		AccountType:   userType,
		CardNumber:    cardNumber,
		CardExpiry:    expiryStr,
		CardStatus:    cardStatus,
		UserStatus:    userStatus,
	}, nil
}

// GetRecentTransactions retrieves the 5 most recent transactions for the dashboard view.
func (r *Repository) GetRecentTransactions(ctx context.Context, username string) ([]Transaction, error) {
	txnQuery := `
    (SELECT 
        t.transaction_id, 
        t.description, 
        t.created_at, 
        COALESCE(t.transaction_type, '') AS transaction_type, 
        t.amount, 
        COALESCE(t.terminal_id, '') AS terminal_id, 
        COALESCE(t.status, '') AS status, 
        m.business_name, 
        m.merchant_id, 
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
        NULL AS business_name, 
        NULL AS merchant_id, 
        0 AS points_earned
    FROM user_activity_logs ual
    JOIN users u ON ual.user_id = u.user_id
    WHERE u.username = ?)
    ORDER BY created_at DESC LIMIT 5
`
	rows, err := r.store.QueryContext(ctx, txnQuery, username, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var createdAt string
		var description sql.NullString
		var businessName sql.NullString
		var merchantId sql.NullString
		var pointsEarned decimal.Decimal

		if err := rows.Scan(
			&t.TransactionID,
			&description,
			&createdAt,
			&t.Type,
			&t.Amount,
			&t.TerminalID,
			&t.Status,
			&businessName,
			&merchantId,
			&pointsEarned,
		); err != nil {
			continue
		}
		t.Date = formatDate(createdAt)
		t.Time = formatTime(createdAt)
		if description.Valid {
			t.Description = description.String
		}
		if businessName.Valid && businessName.String != "" {
			t.MerchantName = businessName.String
		} else {
			t.MerchantName = t.Description
		}
		if merchantId.Valid {
			t.MerchantID = merchantId.String
		} else {
			t.MerchantID = "N/A"
		}
		t.PointsEarned = pointsEarned
		transactions = append(transactions, t)
	}

	return transactions, nil
}

// GetTransactionsList retrieves all transactions associated with the user.
func (r *Repository) GetTransactionsList(ctx context.Context, username string) ([]TxnResponse, error) {
	txnQuery := `
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
			JOIN users u ON t.user_id = u.user_id
			LEFT JOIN cards c ON u.user_id = c.user_id
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
				COALESCE(c.card_number, '') AS card_number
			FROM user_activity_logs ual
			JOIN users u ON ual.user_id = u.user_id
			LEFT JOIN cards c ON u.user_id = c.user_id
			WHERE u.username = ?)
			ORDER BY created_at DESC
		`
	rows, err := r.store.QueryContext(ctx, txnQuery, username, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []TxnResponse
	for rows.Next() {
		var t TxnResponse
		var createdAt string
		var cardNum string

		if err := rows.Scan(
			&t.TransactionID,
			&t.TerminalID,
			&createdAt,
			&t.Type,
			&t.Amount,
			&t.Status,
			&t.Description,
			&t.MerchantName,
			&t.MerchantID,
			&t.PointsEarned,
			&cardNum,
		); err != nil {
			continue
		}

		t.Date = formatDate(createdAt)
		t.Time = formatTime(createdAt)

		switch t.Type {
		case "payment":
			t.Sender = cardNum
			t.Receiver = t.MerchantName
		case "topup":
			t.Sender = "Top-Up Gateway"
			t.Receiver = cardNum
		default:
			t.Sender = cardNum
			t.Receiver = "System"
		}

		transactions = append(transactions, t)
	}

	return transactions, nil
}

// GetUserIDByUsername retrieves the user_id column from users table by username.
func (r *Repository) GetUserIDByUsername(ctx context.Context, username string) (string, error) {
	var userID string
	err := r.store.QueryRowContext(ctx, "SELECT user_id FROM users WHERE username = ?", username).Scan(&userID)
	return userID, err
}

// GetCardDetails retrieves balance and card number for a given user_id.
func (r *Repository) GetCardDetails(ctx context.Context, userID string) (float64, string, error) {
	var balance float64
	var cardNumber string
	err := r.store.QueryRowContext(ctx, "SELECT balance, card_number FROM cards WHERE user_id = ?", userID).Scan(&balance, &cardNumber)
	return balance, cardNumber, err
}

// UpdateCardStatus updates the status column in cards table for a given user_id.
func (r *Repository) UpdateCardStatus(ctx context.Context, userID, status string) error {
	_, err := r.store.ExecContext(ctx, "UPDATE cards SET status = ? WHERE user_id = ?", status, userID)
	return err
}

// RequestCardReplacement atomically deducts the replacement fee and blocks the current card.
func (r *Repository) RequestCardReplacement(ctx context.Context, userID, cardNumber string, fee float64) error {
	tx, err := r.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, "UPDATE cards SET balance = balance - ?, status = 'blocked' WHERE user_id = ? AND balance >= ?", fee, userID, fee)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return ErrInsufficientBalance
	}

	txnID := fmt.Sprintf("REP-%s-%d", cardNumber, time.Now().Unix())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO transactions (transaction_id, card_number, user_id, transaction_type, amount, status, description)
		VALUES (?, ?, ?, 'payment', ?, 'completed', 'Card Replacement Fee')
	`, txnID, cardNumber, userID, fee)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetPasswordHash retrieves the bcrypt password hash for a user.
func (r *Repository) GetPasswordHash(ctx context.Context, username string) (string, error) {
	var currentHash string
	err := r.store.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE username = ?", username).Scan(&currentHash)
	return currentHash, err
}

// UpdatePassword sets a new password hash for the given user.
func (r *Repository) UpdatePassword(ctx context.Context, username, newHash string) error {
	_, err := r.store.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE username = ?", newHash, username)
	return err
}

// GetEmailAndName retrieves the email and name for a given username.
func (r *Repository) GetEmailAndName(ctx context.Context, username string) (string, string, error) {
	var email, name string
	err := r.store.QueryRowContext(ctx, "SELECT email, name FROM users WHERE username = ?", username).Scan(&email, &name)
	return email, name, err
}

// UpdateProfile executes a dynamic SQL update for profile fields.
func (r *Repository) UpdateProfile(ctx context.Context, query string, args []any) error {
	_, err := r.store.ExecContext(ctx, query, args...)
	return err
}

// VerifyEmailToken validates a pending email verification token.
func (r *Repository) VerifyEmailToken(ctx context.Context, token string) (string, string, error) {
	var username, pendingEmail string
	err := r.store.QueryRowContext(ctx, "SELECT username, pending_email FROM users WHERE email_verification_token = ?", token).Scan(&username, &pendingEmail)
	return username, pendingEmail, err
}

// ApplyVerifiedEmail updates the user's email and clears verification fields.
func (r *Repository) ApplyVerifiedEmail(ctx context.Context, username, newEmail string) error {
	_, err := r.store.ExecContext(ctx, "UPDATE users SET email = ?, pending_email = NULL, email_verification_token = NULL WHERE username = ?", newEmail, username)
	return err
}

// GetUserCardAndEmail retrieves the active card number and email for a given username.
func (r *Repository) GetUserCardAndEmail(ctx context.Context, username string) (string, string, error) {
	var cardNumber, email string
	query := `
		SELECT c.card_number, u.email 
		FROM cards c 
		JOIN users u ON c.user_id = u.user_id 
		WHERE u.username = ? LIMIT 1
	`
	err := r.store.QueryRowContext(ctx, query, username).Scan(&cardNumber, &email)
	return cardNumber, email, err
}

// CreatePendingTopUp creates a pending record in the top_ups table.
func (r *Repository) CreatePendingTopUp(ctx context.Context, topupID, cardNumber string, topupAmount, feeAmount decimal.Decimal) error {
	queryTopUp := `INSERT INTO top_ups (topup_id, card_number, amount, convenience_fee, gateway_cost, payment_method, handled_by, external_id, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.store.ExecContext(ctx, queryTopUp, topupID, cardNumber, topupAmount, feeAmount, 0.0, "xendit", "payment gateway", topupID, "pending")
	return err
}

// ProcessPaidTopUp updates card balance, marks topup completed, and writes transaction logs atomically.
func (r *Repository) ProcessPaidTopUp(ctx context.Context, externalID string, payload XenditWebhookPayload) (string, error) {
	var affectedUsername string

	err := r.store.ExecTx(ctx, func(tx *sql.Tx) error {
		var cardNumber string
		var convenienceFee float64
		var currentStatus string
		var amount decimal.Decimal

		err := tx.QueryRowContext(ctx, `SELECT card_number, amount, convenience_fee, status FROM top_ups WHERE topup_id = ? FOR UPDATE`, externalID).Scan(&cardNumber, &amount, &convenienceFee, &currentStatus)
		if err != nil {
			log.Println("ProcessPaidTopUp: record not found or invalid external_id:", err)
			return nil
		}

		if currentStatus == "completed" {
			log.Println("ProcessPaidTopUp: top-up already completed, skipping.")
			return nil
		}

		if _, err := tx.ExecContext(ctx, `UPDATE cards SET balance = balance + ? WHERE card_number = ?`, amount, cardNumber); err != nil {
			return fmt.Errorf("failed to update card balance: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `UPDATE top_ups SET status = 'completed' WHERE topup_id = ?`, externalID); err != nil {
			return fmt.Errorf("failed to update top_ups status: %w", err)
		}

		res, err := tx.ExecContext(ctx, `UPDATE transactions SET status = 'completed', description = 'Successful topup via Xendit' WHERE card_number = ? AND transaction_type = 'topup' AND status = 'pending' AND amount = ? ORDER BY created_at DESC LIMIT 1`, cardNumber, amount)
		if err != nil {
			return fmt.Errorf("failed to update transactions status: %w", err)
		}

		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			transactionID := fmt.Sprintf("TX-%d", time.Now().UnixNano())
			var userID string
			_ = tx.QueryRowContext(ctx, "SELECT user_id FROM cards WHERE card_number = ?", cardNumber).Scan(&userID)
			queryTx := `INSERT INTO transactions (transaction_id, card_number, user_id, merchant_id, terminal_id, transaction_type, amount, service_fee, processed_by, description, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
			if _, err := tx.ExecContext(ctx, queryTx, transactionID, cardNumber, userID, "xendit", "xendit", "topup", amount, convenienceFee, "xendit", "Successful topup via Xendit", "completed"); err != nil {
				return fmt.Errorf("failed to insert transaction: %w", err)
			}
		}

		_ = tx.QueryRowContext(ctx, `
			SELECT u.username 
			FROM users u 
			JOIN cards c ON u.user_id = c.user_id 
			JOIN top_ups tu ON c.card_number = tu.card_number 
			WHERE tu.topup_id = ?
		`, externalID).Scan(&affectedUsername)

		return nil
	})

	return affectedUsername, err
}

// ProcessNonPaidTopUp updates the top_ups status and cleans up any pending transactions if failed/expired/canceled.
func (r *Repository) ProcessNonPaidTopUp(ctx context.Context, externalID, status string) (string, error) {
	_, _ = r.store.ExecContext(ctx, `UPDATE top_ups SET status = ? WHERE topup_id = ?`, strings.ToLower(status), externalID)

	var cardNumber string
	var amount decimal.Decimal
	_ = r.store.QueryRowContext(ctx, `SELECT card_number, amount FROM top_ups WHERE topup_id = ?`, externalID).Scan(&cardNumber, &amount)

	if cardNumber != "" {
		_, _ = r.store.ExecContext(ctx, `DELETE FROM transactions WHERE card_number = ? AND transaction_type = 'topup' AND status = 'pending' AND amount = ? LIMIT 1`, cardNumber, amount)
	}

	var username string
	_ = r.store.QueryRowContext(ctx, `
		SELECT u.username 
		FROM users u 
		JOIN cards c ON u.user_id = c.user_id 
		JOIN top_ups tu ON c.card_number = tu.card_number 
		WHERE tu.topup_id = ?
	`, externalID).Scan(&username)

	return username, nil
}

// Helpers
func formatDate(dbTime string) string {
	t, err := time.Parse("2006-01-02 15:04:05", dbTime)
	if err == nil {
		return t.Format("Jan _2, 2006")
	}
	t2, err := time.Parse(time.RFC3339, dbTime)
	if err == nil {
		return t2.Format("Jan _2, 2006")
	}
	if len(dbTime) >= 10 {
		return dbTime[:10]
	}
	return dbTime
}

func formatTime(dbTime string) string {
	t, err := time.Parse("2006-01-02 15:04:05", dbTime)
	if err == nil {
		return t.Format("03:04 PM")
	}
	t2, err := time.Parse(time.RFC3339, dbTime)
	if err == nil {
		return t2.Format("03:04 PM")
	}
	if len(dbTime) > 10 {
		return dbTime[11:16]
	}
	return ""
}
