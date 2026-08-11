package merchant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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

// Get the merchant details by username.
// This function retrieves the merchant's account summary, including profile, details, bank details, and documents.
func (r *Repository) GetMerchantAccountData(ctx context.Context, username string) (*MerchantDetails, error) {
	var merchant MerchantDetails

	err := r.store.QueryRow(`
		SELECT 
            m.merchant_id, 
            COALESCE(m.status, ''), 
            COALESCE(DATE_FORMAT(m.created_at, '%M %d, %Y'), '') as created_at,
            -- Business Info
            COALESCE(m.business_name, ''), 
            COALESCE(m.business_type, ''), 
            COALESCE(m.business_email, ''), 
            COALESCE(m.business_phone, ''), 
            COALESCE(m.business_address, ''),
            -- Location Info
            COALESCE(m.city, ''),
            COALESCE(m.postal_code, ''), 
            -- Bank Info
            COALESCE(m.settlement_account_name, ''), 
            COALESCE(m.settlement_bank_name, ''), 
            COALESCE(m.settlement_account_number, ''),
            -- Document Info
			COALESCE(m.business_document, ''),
            COALESCE(m.bir_document, ''),
            COALESCE(m.valid_id, ''),
			COALESCE(m.bank_document, ''),
            COALESCE(m.document_status, ''),
            COALESCE(m.message, '')
        FROM merchants m
        JOIN users u ON m.user_id = u.user_id
        WHERE u.username = ?
    `, username).Scan(
		&merchant.merchantID, &merchant.accountStatus, &merchant.createdAtStr, &merchant.businessName,
		&merchant.businessType, &merchant.businessEmail, &merchant.businessPhone,
		&merchant.businessAddress, &merchant.city, &merchant.postalCode, &merchant.accName,
		&merchant.bankName, &merchant.accNumber, &merchant.businessDoc,
		&merchant.birDoc, &merchant.validID, &merchant.bankDoc, &merchant.docStatus, &merchant.docMessage,
	)

	if err != nil {
		log.Printf("GetMerchantAccountData: failed to retrieve merchant data for username %s: %v", username, err)
		return nil, err
	}
	return &merchant, nil
}

// GetMerchantBankDetails retrieves the merchant ID, user ID, and existing account number for a given username.
func (r *Repository) GetMerchantBankDetails(ctx context.Context, username string) (merchantID, userID string, existingAccNumber *string, err error) {
	err = r.store.QueryRowContext(ctx,
		`SELECT merchant_id, settlement_account_number, user_id
        FROM merchants
        WHERE user_id = (SELECT user_id FROM users WHERE username=?)`, username).
		Scan(&merchantID, &existingAccNumber, &userID)
	if err != nil {
		return "", "", nil, fmt.Errorf("get merchant for bank update (username %q): %w", username, err)
	}
	return merchantID, userID, existingAccNumber, nil
}

// Update the bank details of a merchant in the database.
// It updates the settlement bank name, account holder name, and account number for the specified merchant ID.
func (r *Repository) UpdateBankDetails(ctx context.Context, merchantID string, bank BusinessBankDetails) error {
	_, err := r.store.ExecContext(ctx,
		`UPDATE merchants 
        SET settlement_bank_name=?,
        settlement_account_name=?, settlement_account_number=? WHERE merchant_id = ?`,
		bank.BankName, bank.AccountHolderName, bank.AccountNumber, merchantID)
	if err != nil {
		return fmt.Errorf("update bank details (merchant %q): %w", merchantID, err)
	}
	return nil
}

// Insert Bank Update Log into user_activity_logs table to record the update action performed by the merchant.
// It logs the user ID, activity type, channel, status, and description of the action.
func (r *Repository) InsertBankUpdateLog(ctx context.Context, userID, activityType, channel, status, description string) error {
	_, err := r.store.ExecContext(ctx,
		`INSERT INTO user_activity_logs (user_id, activity_type, channel, status, description)
		VALUES (?, 'profile_update', 'in_app', 'completed', 'Settlement bank details were updated by the merchant.')`,
		userID)
	if err != nil {
		return fmt.Errorf("log activity (user %q): %w", userID, err)
	}
	return nil
}

// GetDocumentPath fetches the currently stored path for a given document column.
func (r *Repository) GetDocumentPath(ctx context.Context, username, column string) (*string, error) {
	var path *string
	query := fmt.Sprintf(`
		SELECT %s FROM merchants
		WHERE user_id = (SELECT user_id FROM users WHERE username = ?)`, column)

	if err := r.store.QueryRowContext(ctx, query, username).Scan(&path); err != nil {
		return nil, fmt.Errorf("get document path (column %q, username %q): %w", column, username, err)
	}
	return path, nil
}

// UpdateDocumentPath stores the new document path and resets document_status to Pending.
func (r *Repository) UpdateDocumentPath(ctx context.Context, username, column, dbPath string) error {
	query := fmt.Sprintf(`
		UPDATE merchants SET %s = ?, document_status = 'Pending'
		WHERE user_id = (SELECT user_id FROM users WHERE username = ?)`, column)

	if _, err := r.store.ExecContext(ctx, query, dbPath, username); err != nil {
		return fmt.Errorf("update document path (column %q, username %q): %w", column, username, err)
	}
	return nil
}

// MerchantExists checks whether a merchant account exists for the given username.
func (r *Repository) MerchantExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.store.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM merchants 
			WHERE user_id = (SELECT user_id FROM users WHERE username = ?)
		)`, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check merchant exists (username %q): %w", username, err)
	}
	return exists, nil
}

// MerchantCore holds the essential identifying and settlement information for a merchant.
type MerchantCore struct {
	MerchantID        string
	SettlementBank    *string
	SettlementAccount *string
}

// GetMerchantAccountInfo retrieves the user account status and role for a given merchant.
func (r *Repository) GetMerchantAccountInfo(ctx context.Context, merchantID string) (status, role string, err error) {
	err = r.store.QueryRowContext(ctx, `
		SELECT u.status, u.role
		FROM users u
		JOIN merchants m ON u.user_id = m.user_id
		WHERE m.merchant_id = ? LIMIT 1`, merchantID).Scan(&status, &role)
	if err != nil {
		return "", "", fmt.Errorf("get merchant account info (merchant %q): %w", merchantID, err)
	}
	return status, role, nil
}

// GetMerchantByUsername fetches the core merchant identifiers based on the associated username.
func (r *Repository) GetMerchantByUsername(ctx context.Context, username string) (*MerchantCore, error) {
	var m MerchantCore
	err := r.store.QueryRowContext(ctx,
		`SELECT m.merchant_id, m.settlement_bank_name, m.settlement_account_number
		FROM merchants m
		JOIN users u ON m.user_id = u.user_id
		WHERE u.username = ?
		LIMIT 1`, username).Scan(&m.MerchantID, &m.SettlementBank, &m.SettlementAccount)

	if err != nil {
		return nil, fmt.Errorf("Get merchant core (Username %q): %w", username, err)
	}
	return &m, nil
}

// CountMerchantTransactions returns the total number of transactions processed by the merchant.
func (r *Repository) CountMerchantTransactions(ctx context.Context, merchantID string) (int, error) {
	var count int
	err := r.store.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM transactions WHERE merchant_id = ?`, merchantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count merchant transactions (merchant %q): %w", merchantID, err)
	}
	return count, nil
}

// RevenueSummary holds the aggregated transaction totals for a merchant.
type RevenueSummary struct {
	TotalRevenue    decimal.Decimal
	TotalServiceFee decimal.Decimal
	TotalIncome     decimal.Decimal
}

// GetRevenueSummary calculates the total gross revenue, service fees, and net income for a merchant.
func (r *Repository) GetRevenueSummary(ctx context.Context, merchantID string) (*RevenueSummary, error) {
	var s RevenueSummary
	err := r.store.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(amount), 0), 
			COALESCE(SUM(service_fee), 0), 
			COALESCE(SUM(net_merchant_payout), 0)
		FROM transactions 
		WHERE merchant_id = ? 
		AND transaction_type = 'payment' 
		AND status = 'completed'`, merchantID).Scan(&s.TotalRevenue, &s.TotalServiceFee, &s.TotalIncome)
	if err != nil {
		return nil, fmt.Errorf("get revenue summary (merchant %q): %w", merchantID, err)
	}
	return &s, nil
}

// GetTotalRefunds calculates the total amount refunded from the merchant's transactions.
func (r *Repository) GetTotalRefunds(ctx context.Context, merchantID string) (decimal.Decimal, error) {
	var refunds decimal.Decimal
	err := r.store.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE merchant_id = ?
		AND transaction_type = 'refund'
		AND status = 'completed'`, merchantID).Scan(&refunds)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("get total refunds (merchant %q): %w", merchantID, err)
	}
	return refunds, nil
}

// GetRecentTransactions fetches the most recent transactions and activity logs for the merchant dashboard.
func (r *Repository) GetRecentTransactions(ctx context.Context, merchantID string, limit int) ([]MerchantTransaction, error) {
	rows, err := r.store.QueryContext(ctx, `
		SELECT * FROM (
			SELECT
				transaction_id, COALESCE(card_number, '') AS card_number,
				merchant_id, COALESCE(terminal_id, '') AS terminal_id,
				COALESCE(transaction_type, '') AS transaction_type, COALESCE(amount, 0) AS amount,
				COALESCE(points_earned, 0) AS points_earned, COALESCE(service_fee, 0) AS service_fee,
				COALESCE(net_merchant_payout, 0) AS net_merchant_payout, COALESCE(processed_by, '') AS processed_by,
				COALESCE(status, '') AS status, COALESCE(description, '') AS description, COALESCE(created_at, '') AS created_at
			FROM transactions
			WHERE merchant_id = ?
			UNION ALL
			SELECT 
				CONCAT('LOG-', ual.id) AS transaction_id, '' AS card_number,
				m.merchant_id AS merchant_id, '' AS terminal_id,
				ual.activity_type AS transaction_type, 0.00 AS amount,
				0 AS points_earned, 0.00 AS service_fee,
				0.00 AS net_merchant_payout, ual.user_id AS processed_by,
				ual.status AS status, COALESCE(ual.description, '') AS description, ual.created_at AS created_at
			FROM user_activity_logs ual
			JOIN merchants m ON ual.user_id = m.user_id
			WHERE m.merchant_id = ?
		) AS combined_txns
		ORDER BY created_at DESC
		LIMIT ?`, merchantID, merchantID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent transactions (merchant %q): %w", merchantID, err)
	}
	defer rows.Close()

	transactions := []MerchantTransaction{}
	for rows.Next() {
		var t MerchantTransaction
		if err := rows.Scan(
			&t.TransactionID, &t.CardNumber, &t.MerchantID, &t.TerminalID,
			&t.TransactionType, &t.Amount, &t.Points, &t.ServiceFee,
			&t.NetMerchantPayout, &t.ProcessedBy, &t.Status, &t.Description, &t.Date,
		); err != nil {
			return nil, fmt.Errorf("scan recent transaction row (merchant %q): %w", merchantID, err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent transactions (merchant %q): %w", merchantID, err)
	}
	return transactions, nil
}

// GetMerchantIDByUsername resolves a merchant_id from a username. Shared
// across services that only need the ID (dashboard, income, documents, etc.)
func (r *Repository) GetMerchantIDByUsername(ctx context.Context, username string) (string, error) {
	var merchantID string
	err := r.store.QueryRowContext(ctx, `
		SELECT m.merchant_id 
		FROM merchants m
		JOIN users u ON m.user_id = u.user_id
		WHERE u.username = ?
		LIMIT 1`, username).Scan(&merchantID)
	if err != nil {
		return "", fmt.Errorf("get merchant id (username %q): %w", username, err)
	}
	return merchantID, nil
}

// GetIncomeStats computes the detailed income, revenue, and withdrawal statistics for the merchant.
func (r *Repository) GetIncomeStats(ctx context.Context, merchantID string) (*IncomeStat, error) {
	var (
		totalCollected, unicardFee, totalEarned, totalRefunded,
		earnedThisMonth, refundedThisMonth,
		totalWithdrawn, withdrawnThisMonth decimal.Decimal
	)

	err := r.store.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN transaction_type = 'payment' AND status = 'completed' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'payment' AND status = 'completed' THEN service_fee ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'payment' AND status = 'completed' THEN net_merchant_payout ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'refund' AND status = 'completed' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'payment' AND status = 'completed'
				AND MONTH(created_at) = MONTH(NOW()) 
				AND YEAR(created_at) = YEAR(NOW()) 
				THEN net_merchant_payout ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'refund' AND status = 'completed'
				AND MONTH(created_at) = MONTH(NOW()) 
				AND YEAR(created_at) = YEAR(NOW()) 
				THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'withdrawal' AND status IN ('completed', 'pending') THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN transaction_type = 'withdrawal' AND status IN ('completed', 'pending')
				AND MONTH(created_at) = MONTH(NOW()) 
				AND YEAR(created_at) = YEAR(NOW()) 
				THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE merchant_id = ?`, merchantID).Scan(
		&totalCollected, &unicardFee, &totalEarned, &totalRefunded,
		&earnedThisMonth, &refundedThisMonth,
		&totalWithdrawn, &withdrawnThisMonth,
	)
	if err != nil {
		return nil, fmt.Errorf("get income stats (merchant %q): %w", merchantID, err)
	}

	return &IncomeStat{
		GrossRevenue:     totalCollected,
		PlatformFee:      unicardFee,
		NetRevenue:       totalEarned.Sub(totalRefunded),
		TotalRefunds:     totalRefunded,
		MonthlyNetIncome: earnedThisMonth.Sub(refundedThisMonth),
		MonthlyRefunds:   refundedThisMonth,
		TotalWithdrawn:   totalWithdrawn,
		// AvailableBalance is a derived field left for the service layer to
		// compute, since it depends on NetRevenue and TotalWithdrawn together.
		MonthlyWithdrawn: withdrawnThisMonth,
	}, nil
}

// GetIncomeHistory retrieves a paginated list of completed income and payout transactions.
func (r *Repository) GetIncomeHistory(ctx context.Context, merchantID string, limit int) ([]IncomeHistory, error) {
	rows, err := r.store.QueryContext(ctx, `
		SELECT 
			COALESCE(created_at, ''), description,
			transaction_id, COALESCE(card_number, ''),
			COALESCE(transaction_type, ''), COALESCE(amount, 0),
			COALESCE(net_merchant_payout, 0), COALESCE(service_fee, 0),
			processed_by, terminal_id
		FROM transactions
		WHERE merchant_id = ? AND status = 'completed'
		ORDER BY created_at DESC LIMIT ?`, merchantID, limit)
	if err != nil {
		return nil, fmt.Errorf("get income history (merchant %q): %w", merchantID, err)
	}
	defer rows.Close()

	history := []IncomeHistory{}
	for rows.Next() {
		var row IncomeHistory
		if err := rows.Scan(
			&row.Date, &row.Description, &row.TransactionID, &row.CardNumber,
			&row.TransactionType, &row.Amount, &row.NetIncome, &row.ServiceFee,
			&row.ProcessedBy, &row.TerminalID,
		); err != nil {
			return nil, fmt.Errorf("scan income history row (merchant %q): %w", merchantID, err)
		}
		history = append(history, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate income history (merchant %q): %w", merchantID, err)
	}
	return history, nil
}

// MerchantBankInfo represents the necessary details to process a withdrawal to a merchant's bank.
type MerchantBankInfo struct {
	MerchantID     string
	UserID         string
	SettlementBank *string
	SettlementName *string
	SettlementAcct *string
}

// GetMerchantBankInfo fetches the settlement bank details needed for payouts.
func (r *Repository) GetMerchantBankInfo(ctx context.Context, username string) (*MerchantBankInfo, error) {
	var b MerchantBankInfo
	err := r.store.QueryRowContext(ctx, `
		SELECT m.merchant_id, m.user_id, m.settlement_bank_name, m.settlement_account_name, m.settlement_account_number
		FROM merchants m
		JOIN users u ON m.user_id = u.user_id
		WHERE u.username = ? LIMIT 1`, username).
		Scan(&b.MerchantID, &b.UserID, &b.SettlementBank, &b.SettlementName, &b.SettlementAcct)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMerchantNotFound
		}
		return nil, fmt.Errorf("get merchant bank info (username %q): %w", username, err)
	}
	return &b, nil
}

// GetDailyWithdrawnAmount calculates the total amount withdrawn by the merchant on the current day.
func (r *Repository) GetDailyWithdrawnAmount(ctx context.Context, merchantID string) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.store.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0) 
		FROM transactions 
		WHERE merchant_id = ? AND transaction_type = 'withdrawal' AND DATE(created_at) = CURDATE()`,
		merchantID).Scan(&total)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("get daily withdrawn amount (merchant %q): %w", merchantID, err)
	}
	return total, nil
}

// InsertWithdrawalTransaction creates a new pending transaction record for a merchant withdrawal.
func (r *Repository) InsertWithdrawalTransaction(ctx context.Context, txnID, merchantID, userID, description string, amount, serviceFee decimal.Decimal, status string) error {
	_, err := r.store.ExecContext(ctx, `
		INSERT INTO transactions (
			transaction_id, merchant_id, user_id, transaction_type, amount, status, description, card_number, service_fee
		) VALUES (?, ?, ?, 'withdrawal', ?, ?, ?, NULL, ?)`,
		txnID, merchantID, userID, amount, status, description, serviceFee)
	if err != nil {
		return fmt.Errorf("insert withdrawal transaction (txn %q): %w", txnID, err)
	}
	return nil
}

// UpdateWithdrawalStatus safely updates the status of a withdrawal transaction.
func (r *Repository) UpdateWithdrawalStatus(ctx context.Context, txnID, newStatus, fromStatus string) error {
	res, err := r.store.ExecContext(ctx, `
		UPDATE transactions SET status = ? 
		WHERE transaction_id = ? AND transaction_type = 'withdrawal' AND status = ?`,
		newStatus, txnID, fromStatus)
	if err != nil {
		return fmt.Errorf("update withdrawal status (txn %q): %w", txnID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update withdrawal status (txn %q): %w", txnID, sql.ErrNoRows)
	}
	return nil
}

// InsertTerminalRequest logs a new request from the merchant for a payment terminal.
func (r *Repository) InsertTerminalRequest(ctx context.Context, requestID, merchantID, terminalSN, notes string) error {
	_, err := r.store.ExecContext(ctx, `
		INSERT INTO terminal_requests (request_id, merchant_id, terminal_sn, status, requested_at, notes) 
		VALUES (?, ?, ?, 'pending', CURRENT_TIMESTAMP, ?)`,
		requestID, merchantID, terminalSN, notes)
	if err != nil {
		return fmt.Errorf("insert terminal request (merchant %q): %w", merchantID, err)
	}
	return nil
}

// TransactionFilter defines the criteria for querying merchant transactions.
type TransactionFilter struct {
	Search string
	Type   string
	Sort   string // "asc" or "desc"
	Limit  int
}

// GetTransactions fetches merchant transactions based on the provided filter criteria.
func (r *Repository) GetTransactions(ctx context.Context, merchantID string, f TransactionFilter) ([]MerchantTransaction, error) {
	query := `SELECT * FROM (
		SELECT 
			transaction_id, COALESCE(card_number, '') AS card_number,
			merchant_id, COALESCE(terminal_id, '') AS terminal_id,
			COALESCE(transaction_type, '') AS transaction_type, COALESCE(amount, 0) AS amount,
			COALESCE(points_earned, 0) AS points_earned, COALESCE(service_fee, 0) AS service_fee,
			COALESCE(net_merchant_payout, 0) AS net_merchant_payout, COALESCE(processed_by, '') AS processed_by,
			COALESCE(status, '') AS status, COALESCE(description, '') AS description, COALESCE(created_at, '') AS created_at
		FROM transactions 
		WHERE merchant_id = ?
		UNION ALL
		SELECT 
			CONCAT('LOG-', ual.id) AS transaction_id, '' AS card_number,
			m.merchant_id AS merchant_id, '' AS terminal_id,
			ual.activity_type AS transaction_type, 0.00 AS amount,
			0 AS points_earned, 0.00 AS service_fee,
			0.00 AS net_merchant_payout, ual.user_id AS processed_by,
			ual.status AS status, COALESCE(ual.description, '') AS description, ual.created_at AS created_at
		FROM user_activity_logs ual
		JOIN merchants m ON ual.user_id = m.user_id
		WHERE m.merchant_id = ?
	) AS combined_txns WHERE 1=1`

	args := []any{merchantID, merchantID}

	if f.Type != "" && f.Type != "all" {
		query += ` AND transaction_type = ?`
		args = append(args, f.Type)
	}
	if f.Search != "" {
		query += ` AND (description LIKE ? OR transaction_id LIKE ?)`
		term := "%" + f.Search + "%"
		args = append(args, term, term)
	}
	if f.Sort == "asc" {
		query += ` ORDER BY created_at ASC`
	} else {
		query += ` ORDER BY created_at DESC`
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query += ` LIMIT ?`
	args = append(args, limit)

	rows, err := r.store.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get transactions (merchant %q): %w", merchantID, err)
	}
	defer rows.Close()

	transactions := []MerchantTransaction{}
	for rows.Next() {
		var t MerchantTransaction
		if err := rows.Scan(
			&t.TransactionID, &t.CardNumber, &t.MerchantID, &t.TerminalID,
			&t.TransactionType, &t.Amount, &t.Points, &t.ServiceFee,
			&t.NetMerchantPayout, &t.ProcessedBy, &t.Status, &t.Description, &t.Date,
		); err != nil {
			return nil, fmt.Errorf("scan transaction row (merchant %q): %w", merchantID, err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions (merchant %q): %w", merchantID, err)
	}
	return transactions, nil
}
