package admin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	structs "project-tap/internal/pkg/structs"
	"project-tap/internal/pkg/xenditclient"
)

// Service holds all admin business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new admin Service with the required dependencies.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

// GetDashboardData aggregates system-wide metrics for the admin overview dashboard.
func (s *Service) GetDashboardData() (structs.AdminDashboardData, error) {
	store := s.repo.store
	var data structs.AdminDashboardData

	if err := store.QueryRow(`SELECT COALESCE(SUM(service_fee), 0.00) FROM transactions WHERE transaction_type IN ('payment', 'topup', 'withdrawal')`).Scan(&data.GrossRevenue); err != nil {
		return data, fmt.Errorf("gross revenue: %w", err)
	}

	if err := store.QueryRow(`SELECT COALESCE(SUM(CASE WHEN transaction_type IN ('payment','topup','withdrawal') THEN service_fee WHEN transaction_type IN ('refund','reversal') THEN -service_fee ELSE 0 END), 0.00) FROM transactions`).Scan(&data.NetRevenue); err != nil {
		return data, fmt.Errorf("net revenue: %w", err)
	}

	if err := store.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'customer'`).Scan(&data.TotalUsers); err != nil {
		return data, fmt.Errorf("total users: %w", err)
	}

	if err := store.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&data.TotalCards); err != nil {
		return data, fmt.Errorf("total cards: %w", err)
	}

	if err := store.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='pending_approval' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN status='suspended' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN status='rejected' THEN 1 ELSE 0 END),0) FROM merchants`).
		Scan(&data.TotalMerchants, &data.PendingMerchants, &data.SuspendedMerchants, &data.RejectedMerchants); err != nil {
		return data, fmt.Errorf("merchant stats: %w", err)
	}

	if err := store.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='active' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN status='inactive' THEN 1 ELSE 0 END),0) FROM terminals`).
		Scan(&data.TotalTerminals, &data.ActiveTerminals, &data.InactiveTerminals); err != nil {
		return data, fmt.Errorf("terminal stats: %w", err)
	}

	rows, err := store.Query(`SELECT merchant_id, business_name, business_type, owner_name, business_email, business_phone, status, created_at FROM merchants ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		return data, fmt.Errorf("recent merchants: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m structs.Merchant
		if err := rows.Scan(&m.MerchantID, &m.BusinessName, &m.BusinessType, &m.OwnerName, &m.Email, &m.Phone, &m.Status, &m.CreatedAt); err != nil {
			log.Printf("GetDashboardData: scan merchant: %v", err)
			continue
		}
		data.Merchants = append(data.Merchants, m)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Merchants
// ---------------------------------------------------------------------------

// GetMerchants fetches a paginated, filtered list of merchants and their assigned terminals.
func (s *Service) GetMerchants(page, limit int, search, sortOrder, category, status string) (PaginatedMerchantsResult, error) {
	store := s.repo.store

	baseQuery := `FROM merchants`
	var args []any
	var conditions []string

	if search != "" {
		conditions = append(conditions, `(business_name LIKE ? OR owner_name LIKE ? OR merchant_id LIKE ?)`)
		p := "%" + search + "%"
		args = append(args, p, p, p)
	}
	if category != "" {
		conditions = append(conditions, `business_type = ?`)
		args = append(args, category)
	}
	if status != "" {
		conditions = append(conditions, `status = ?`)
		args = append(args, status)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := store.QueryRow("SELECT COUNT(*) "+baseQuery+where, args...).Scan(&total); err != nil {
		return PaginatedMerchantsResult{}, fmt.Errorf("count merchants: %w", err)
	}

	order := " ORDER BY created_at DESC"
	switch strings.ToLower(sortOrder) {
	case "asc":
		order = " ORDER BY created_at ASC"
	case "name_asc":
		order = " ORDER BY business_name ASC"
	case "name_desc":
		order = " ORDER BY business_name DESC"
	}

	offset := (page - 1) * limit
	query := `SELECT merchant_id, business_name, business_type, owner_name, business_email, business_phone, status, created_at ` + baseQuery + where + order + ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := store.Query(query, args...)
	if err != nil {
		return PaginatedMerchantsResult{}, fmt.Errorf("query merchants: %w", err)
	}
	defer rows.Close()

	var merchants []structs.Merchant
	for rows.Next() {
		var m structs.Merchant
		if err := rows.Scan(&m.MerchantID, &m.BusinessName, &m.BusinessType, &m.OwnerName, &m.Email, &m.Phone, &m.Status, &m.CreatedAt); err != nil {
			log.Printf("GetMerchants: scan: %v", err)
			continue
		}
		merchants = append(merchants, m)
	}
	if merchants == nil {
		merchants = []structs.Merchant{}
	} else {
		merchants = s.attachTerminals(merchants)
	}

	return PaginatedMerchantsResult{Merchants: merchants, TotalItems: total, Page: page, Limit: limit}, nil
}

// attachTerminals populates the Terminals field for a batch of merchants.
func (s *Service) attachTerminals(merchants []structs.Merchant) []structs.Merchant {
	store := s.repo.store
	ids := make([]string, len(merchants))
	args := make([]any, len(merchants))
	for i, m := range merchants {
		ids[i] = "?"
		args[i] = m.MerchantID
	}

	q := fmt.Sprintf(`SELECT m.merchant_id, t.terminal_id, t.terminal_sn, t.device_name, t.status FROM terminals t JOIN merchants m ON t.merchant_id = m.merchant_id WHERE m.merchant_id IN (%s)`, strings.Join(ids, ","))
	rows, err := store.Query(q, args...)
	if err != nil {
		log.Printf("attachTerminals: query: %v", err)
		for i := range merchants {
			merchants[i].Terminals = []structs.Terminal{}
		}
		return merchants
	}
	defer rows.Close()

	termMap := make(map[string][]structs.Terminal)
	for rows.Next() {
		var mID string
		var t structs.Terminal
		if err := rows.Scan(&mID, &t.TerminalID, &t.TerminalSN, &t.DeviceName, &t.Status); err == nil {
			termMap[mID] = append(termMap[mID], t)
		}
	}
	for i := range merchants {
		if t, ok := termMap[merchants[i].MerchantID]; ok {
			merchants[i].Terminals = t
		} else {
			merchants[i].Terminals = []structs.Terminal{}
		}
	}
	return merchants
}

// GetMerchantByID retrieves the complete profile and terminal list for a specific merchant.
func (s *Service) GetMerchantByID(merchantID string) (MerchantDetailsData, error) {
	store := s.repo.store
	var m MerchantDetailsData
	var commRate sql.NullFloat64
	var setBank, setName, setAcct, regNum, dtiDoc, birDoc, otherDoc, city, postal, docStatus, busAddress, busPhone, busType sql.NullString

	err := store.QueryRow(`
		SELECT merchant_id, user_id, business_name, business_type, business_registration_number,
		       business_address, city, postal_code, owner_name, business_email, business_phone, status,
		       commission_rate, settlement_bank_name, settlement_account_name,
		       settlement_account_number, created_at,
		       business_document, bir_document, valid_id, document_status
		FROM merchants WHERE merchant_id = ?`, merchantID).Scan(
		&m.MerchantID, &m.UserID, &m.BusinessName, &busType, &regNum,
		&busAddress, &city, &postal, &m.OwnerName, &m.BusinessEmail, &busPhone, &m.Status,
		&commRate, &setBank, &setName, &setAcct, &m.CreatedAt,
		&dtiDoc, &birDoc, &otherDoc, &docStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, errors.New("merchant not found")
		}
		return m, fmt.Errorf("query merchant: %w", err)
	}

	if busType.Valid {
		m.BusinessType = busType.String
	}
	if busAddress.Valid {
		m.BusinessAddress = busAddress.String
	}
	if busPhone.Valid {
		m.BusinessPhone = busPhone.String
	}
	if regNum.Valid {
		m.RegistrationNum = regNum.String
	}
	if city.Valid {
		m.City = city.String
	}
	if postal.Valid {
		m.PostalCode = postal.String
	}
	if commRate.Valid {
		m.CommissionRate = commRate.Float64
	}
	if setBank.Valid {
		m.SettlementBank = setBank.String
	}
	if setName.Valid {
		m.SettlementName = setName.String
	}
	if setAcct.Valid {
		m.SettlementAcct = setAcct.String
	}
	if dtiDoc.Valid {
		m.BusinessDocument = dtiDoc.String
	}
	if birDoc.Valid {
		m.BirDocument = birDoc.String
	}
	if otherDoc.Valid {
		m.ValidId = otherDoc.String
	}
	if docStatus.Valid {
		m.DocumentStatus = docStatus.String
	}

	termRows, err := store.Query(`SELECT terminal_id, terminal_sn, device_name, status FROM terminals WHERE merchant_id = ?`, merchantID)
	if err != nil {
		m.Terminals = []structs.Terminal{}
	} else {
		defer termRows.Close()
		for termRows.Next() {
			var t structs.Terminal
			if err := termRows.Scan(&t.TerminalID, &t.TerminalSN, &t.DeviceName, &t.Status); err == nil {
				m.Terminals = append(m.Terminals, t)
			}
		}
	}
	return m, nil
}

// ApproveMerchant activates a merchant account and assigns them a physical payment terminal.
func (s *Service) ApproveMerchant(ctx context.Context, merchantID, adminUsername string, req ApproveMerchantRequest) error {
	store := s.repo.store
	tx, err := store.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var adminUserID string
	if err := tx.QueryRow("SELECT user_id FROM users WHERE username = ?", adminUsername).Scan(&adminUserID); err != nil {
		return errors.New("admin user not found")
	}

	var merchantUserID, merchantEmail, ownerName, businessName, businessAddress string
	if err := tx.QueryRow("SELECT user_id, business_email, owner_name, business_name, business_address FROM merchants WHERE merchant_id = ?", merchantID).
		Scan(&merchantUserID, &merchantEmail, &ownerName, &businessName, &businessAddress); err != nil {
		return errors.New("merchant not found")
	}

	_, err = tx.Exec(`UPDATE merchants SET status='active', document_status='approved', message='Congratulations! Your UniCard Merchant Account is now fully active.', commission_rate=2.00, approved_by=?, approved_at=CURRENT_TIMESTAMP WHERE merchant_id=?`, adminUserID, merchantID)
	if err != nil {
		return fmt.Errorf("update merchant: %w", err)
	}

	_, err = tx.Exec("UPDATE users SET status='active' WHERE user_id=?", merchantUserID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Resolve terminal SN
	assignSN := req.TerminalSn
	var requestID string
	if assignSN == "" {
		_ = tx.QueryRow("SELECT request_id, terminal_sn FROM terminal_requests WHERE merchant_id=? AND status='pending' ORDER BY requested_at DESC LIMIT 1", merchantID).Scan(&requestID, &assignSN)
		if assignSN == "" {
			_ = tx.QueryRow("SELECT terminal_sn FROM terminals WHERE merchant_id IS NULL AND status='inactive' LIMIT 1").Scan(&assignSN)
		}
	}
	if assignSN == "" {
		return errors.New("no terminal selected or available")
	}

	var deviceName string
	if err := tx.QueryRow("SELECT device_name FROM terminals WHERE terminal_sn=?", assignSN).Scan(&deviceName); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("selected terminal was not found")
		}
		return fmt.Errorf("read terminal: %w", err)
	}

	_, err = tx.Exec("UPDATE terminals SET merchant_id=?, device_name=?, location_details=?, status='active' WHERE terminal_sn=?", merchantID, deviceName, businessAddress, assignSN)
	if err != nil {
		return fmt.Errorf("assign terminal: %w", err)
	}

	if requestID != "" {
		_, _ = tx.Exec("UPDATE terminal_requests SET status='approved', handled_by=?, handled_at=CURRENT_TIMESTAMP WHERE request_id=?", adminUserID, requestID)
	}

	_, err = tx.Exec(`INSERT INTO user_activity_logs (user_id, activity_type, channel, status, description) VALUES (?, 'onboarding', 'in_app', 'completed', 'Welcome to UniCard! Your merchant account is now approved and ready to accept transactions.')`, merchantUserID)
	if err != nil {
		return fmt.Errorf("activity log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	go SendMerchantApprovedEmail(merchantEmail, ownerName)
	return nil
}

// RejectMerchant declines a merchant application and deactivates the associated user account.
func (s *Service) RejectMerchant(merchantID string, req RejectMerchantRequest) error {
	store := s.repo.store
	if req.Reason == "" {
		return errors.New("rejection reason is required")
	}

	tx, err := store.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var merchantUserID, merchantEmail, ownerName string
	if err := tx.QueryRow("SELECT user_id, business_email, owner_name FROM merchants WHERE merchant_id=?", merchantID).Scan(&merchantUserID, &merchantEmail, &ownerName); err != nil {
		return errors.New("merchant not found")
	}

	if _, err = tx.Exec("UPDATE merchants SET status='rejected', document_status='rejected', message=? WHERE merchant_id=?", req.Reason, merchantID); err != nil {
		return fmt.Errorf("update merchant: %w", err)
	}
	if _, err = tx.Exec("UPDATE users SET status='inactive' WHERE user_id=?", merchantUserID); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	go SendMerchantRejectedEmail(merchantEmail, ownerName, req.Reason)
	return nil
}

// SuspendMerchant temporarily disables a merchant account and its associated user access.
func (s *Service) SuspendMerchant(merchantID string, req SuspendMerchantRequest) error {
	store := s.repo.store
	if req.Reason == "" {
		return errors.New("suspension reason is required")
	}

	tx, err := store.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var merchantUserID, merchantEmail, ownerName string
	if err := tx.QueryRow("SELECT user_id, business_email, owner_name FROM merchants WHERE merchant_id=?", merchantID).Scan(&merchantUserID, &merchantEmail, &ownerName); err != nil {
		return errors.New("merchant not found")
	}

	if _, err = tx.Exec("UPDATE merchants SET status='suspended', message=? WHERE merchant_id=?", req.Reason, merchantID); err != nil {
		return fmt.Errorf("update merchant: %w", err)
	}
	if _, err = tx.Exec("UPDATE users SET status='inactive' WHERE user_id=?", merchantUserID); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	go SendMerchantSuspendedEmail(merchantEmail, ownerName, req.Reason)
	return nil
}

// DeleteMerchant permanently removes a merchant profile and unassigns any linked terminals.
func (s *Service) DeleteMerchant(merchantID string) error {
	store := s.repo.store
	tx, err := store.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var merchantUserID, ownerName, merchantEmail string
	if err := tx.QueryRow("SELECT user_id, owner_name, business_email FROM merchants WHERE merchant_id=?", merchantID).Scan(&merchantUserID, &ownerName, &merchantEmail); err != nil {
		return errors.New("merchant not found")
	}

	if _, err = tx.Exec("UPDATE terminals SET merchant_id=NULL, location_details='', status='inactive' WHERE merchant_id=?", merchantID); err != nil {
		return fmt.Errorf("reset terminals: %w", err)
	}
	if _, err = tx.Exec("UPDATE merchants SET status='deleted', message='Your merchant account has been permanently deleted.' WHERE merchant_id=?", merchantID); err != nil {
		return fmt.Errorf("delete merchant: %w", err)
	}
	if _, err = tx.Exec("UPDATE users SET status='inactive' WHERE user_id=?", merchantUserID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	go SendMerchantDeletedEmail(merchantEmail, ownerName)
	return nil
}

// ApproveMerchantDocuments marks a merchant's uploaded verification documents as approved.
func (s *Service) ApproveMerchantDocuments(merchantID string) error {
	_, err := s.repo.store.Exec("UPDATE merchants SET document_status='approved', message='Your newly uploaded documents have been approved.' WHERE merchant_id=?", merchantID)
	return err
}

// AddMerchants processes a batch onboarding request for multiple new merchants.
func (s *Service) AddMerchants(ctx context.Context, reqs []AddMerchantRequest) error {
	store := s.repo.store
	caser := cases.Title(language.English)

	tx, err := store.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	userStmt, err := tx.Prepare(`INSERT INTO users (user_id, username, name, email, phone_number, password_hash, role, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare user stmt: %w", err)
	}
	defer userStmt.Close()

	merchStmt, err := tx.Prepare(`INSERT INTO merchants (merchant_id, business_name, business_type, business_registration_number, business_address, user_id, owner_name, business_email, business_phone, commission_rate, settlement_account_name, settlement_account_number, settlement_bank_name, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare merchant stmt: %w", err)
	}
	defer merchStmt.Close()

	termStmt, err := tx.Prepare(`UPDATE terminals SET merchant_id=?, location_details=?, status='active' WHERE terminal_sn=?`)
	if err != nil {
		return fmt.Errorf("prepare terminal stmt: %w", err)
	}
	defer termStmt.Close()

	for i, req := range reqs {
		req.BusinessName = caser.String(strings.ToLower(strings.TrimSpace(req.BusinessName)))
		req.BusinessAddress = caser.String(strings.ToLower(strings.TrimSpace(req.BusinessAddress)))
		req.OwnerName = caser.String(strings.ToLower(strings.TrimSpace(req.OwnerName)))
		req.SettlementName = caser.String(strings.ToLower(strings.TrimSpace(req.SettlementName)))
		req.BusinessEmail = strings.ToLower(strings.TrimSpace(req.BusinessEmail))
		req.BusinessPhone = strings.TrimSpace(req.BusinessPhone)
		req.TerminalSN = strings.TrimSpace(req.TerminalSN)
		req.DeviceName = strings.TrimSpace(req.DeviceName)

		if msg, ok := ValidateAddMerchantRequest(req, i); !ok {
			return errors.New(msg)
		}

		timestamp := time.Now().Format("01020605")
		nUser, _ := rand.Int(rand.Reader, big.NewInt(10000))
		userID := fmt.Sprintf("UNI-%s%04d", timestamp, nUser.Int64())

		nMerchant, _ := rand.Int(rand.Reader, big.NewInt(10000))
		merchantID := fmt.Sprintf("MCH-%s%04d", timestamp, nMerchant.Int64())

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("TempPass123!"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("merchant #%d: hash password: %w", i+1, err)
		}

		if _, err = userStmt.Exec(userID, req.BusinessEmail, req.OwnerName, req.BusinessEmail, req.BusinessPhone, string(hashedPassword), "merchant_admin", "active"); err != nil {
			return fmt.Errorf("merchant #%d: create user (email or phone exists): %w", i+1, err)
		}

		nReg, _ := rand.Int(rand.Reader, big.NewInt(10000000000))
		regNum := fmt.Sprintf("UCBZ-%s-%010d", time.Now().Format("010205"), nReg.Int64())

		if _, err = merchStmt.Exec(merchantID, req.BusinessName, req.BusinessType, regNum, req.BusinessAddress, userID, req.OwnerName, req.BusinessEmail, req.BusinessPhone, 2.00, req.SettlementName, req.SettlementAccount, req.SettlementBank, "active"); err != nil {
			return fmt.Errorf("merchant #%d: create merchant profile: %w", i+1, err)
		}

		if _, err = termStmt.Exec(userID, req.BusinessAddress, req.TerminalSN); err != nil {
			return fmt.Errorf("merchant #%d: register terminal: %w", i+1, err)
		}
	}

	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Terminals
// ---------------------------------------------------------------------------

// GetTerminals retrieves a paginated and filtered list of all physical payment terminals.
func (s *Service) GetTerminals(page, limit int, search, status, sortOrder string) (PaginatedTerminalsResult, error) {
	store := s.repo.store

	base := `FROM terminals t LEFT JOIN merchants m ON t.merchant_id = m.merchant_id`
	var args []any
	var conditions []string

	if search != "" {
		conditions = append(conditions, `(t.terminal_id LIKE ? OR t.terminal_sn LIKE ? OR m.business_name LIKE ?)`)
		p := "%" + search + "%"
		args = append(args, p, p, p)
	}
	if status != "" {
		conditions = append(conditions, `t.status = ?`)
		args = append(args, status)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := store.QueryRow("SELECT COUNT(*) "+base+where, args...).Scan(&total); err != nil {
		return PaginatedTerminalsResult{}, fmt.Errorf("count terminals: %w", err)
	}

	order := " ORDER BY t.created_at DESC"
	if strings.ToLower(sortOrder) == "asc" {
		order = " ORDER BY t.created_at ASC"
	}

	offset := (page - 1) * limit
	query := `SELECT t.terminal_id, t.terminal_sn, COALESCE(m.business_name,'Unassigned / Inventory'), t.device_name, COALESCE(t.location_details,'Not Set'), t.status ` + base + where + order + ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := store.Query(query, args...)
	if err != nil {
		return PaginatedTerminalsResult{}, fmt.Errorf("query terminals: %w", err)
	}
	defer rows.Close()

	var terminals []structs.Terminal
	for rows.Next() {
		var t structs.Terminal
		if err := rows.Scan(&t.TerminalID, &t.TerminalSN, &t.AssignedMerch, &t.DeviceName, &t.LocationDetails, &t.Status); err != nil {
			log.Printf("GetTerminals: scan: %v", err)
			continue
		}
		terminals = append(terminals, t)
	}
	if terminals == nil {
		terminals = []structs.Terminal{}
	}

	var active, inactive int
	store.QueryRow(`SELECT COUNT(*) FROM terminals WHERE status IN ('active','online')`).Scan(&active)
	store.QueryRow(`SELECT COUNT(*) FROM terminals WHERE status IN ('inactive','offline')`).Scan(&inactive)

	return PaginatedTerminalsResult{
		Terminals: terminals, TotalItems: total, Page: page, Limit: limit,
		ActiveCount: active, InactiveCount: inactive,
	}, nil
}

// AddTerminal registers a new unassigned payment terminal into the system inventory.
func (s *Service) AddTerminal(req AddTerminalRequest) error {
	timestamp := time.Now().Format("01020605")
	n, _ := rand.Int(rand.Reader, big.NewInt(10000))
	terminalID := fmt.Sprintf("TRM-%s%04d", timestamp, n.Int64())

	_, err := s.repo.store.Exec(`INSERT INTO terminals (terminal_id, terminal_sn, merchant_id, device_name, status) VALUES (?, ?, NULL, ?, 'inactive')`, terminalID, req.TerminalSN, req.DeviceName)
	if err != nil {
		return fmt.Errorf("insert terminal: %w", err)
	}
	return nil
}

// GetUnassignedTerminals returns a list of inactive terminals available for merchant assignment.
func (s *Service) GetUnassignedTerminals() ([]UnassignedTerminalData, error) {
	rows, err := s.repo.store.Query(`SELECT terminal_sn, device_name, status FROM terminals WHERE merchant_id IS NULL AND status='inactive'`)
	if err != nil {
		return nil, fmt.Errorf("query unassigned terminals: %w", err)
	}
	defer rows.Close()

	var terminals []UnassignedTerminalData
	for rows.Next() {
		var t UnassignedTerminalData
		if err := rows.Scan(&t.TerminalSN, &t.DeviceName, &t.Status); err != nil {
			log.Printf("GetUnassignedTerminals: scan: %v", err)
			continue
		}
		terminals = append(terminals, t)
	}
	return terminals, nil
}

// ---------------------------------------------------------------------------
// Terminal requests
// ---------------------------------------------------------------------------

// GetTerminalRequests retrieves a paginated list of merchant applications for new hardware.
func (s *Service) GetTerminalRequests(page, limit int, status, search string) (TerminalRequestsResult, error) {
	store := s.repo.store

	// Check table exists first
	var exists int
	if err := store.QueryRow(`SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='unicard' AND TABLE_NAME='terminal_requests' LIMIT 1`).Scan(&exists); err != nil {
		return TerminalRequestsResult{Requests: []TerminalRequest{}, CurrentPage: page, TotalPages: 1}, nil
	}

	base := `FROM terminal_requests tr JOIN merchants m ON tr.merchant_id = m.merchant_id LEFT JOIN terminals t ON tr.terminal_sn = t.terminal_sn`
	var args []any
	var conditions []string

	if status != "" {
		conditions = append(conditions, `tr.status = ?`)
		args = append(args, status)
	}
	if search != "" {
		conditions = append(conditions, `(tr.request_id LIKE ? OR tr.merchant_id LIKE ? OR m.business_name LIKE ? OR tr.terminal_sn LIKE ?)`)
		p := "%" + search + "%"
		args = append(args, p, p, p, p)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := store.QueryRow("SELECT COUNT(*) "+base+where, args...).Scan(&total); err != nil {
		return TerminalRequestsResult{}, fmt.Errorf("count terminal requests: %w", err)
	}

	offset := (page - 1) * limit
	dataQuery := `SELECT tr.id, tr.request_id, tr.merchant_id, tr.terminal_sn, tr.status, tr.requested_at, tr.handled_by, tr.handled_at, tr.notes, m.business_name, m.owner_name, t.device_name ` + base + where + ` ORDER BY tr.requested_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := store.Query(dataQuery, args...)
	if err != nil {
		return TerminalRequestsResult{}, fmt.Errorf("query terminal requests: %w", err)
	}
	defer rows.Close()

	var requests []TerminalRequest
	for rows.Next() {
		var tr TerminalRequest
		var requestedRaw, handledAtRaw []byte
		var handledByNull, notesNull sql.NullString

		if err := rows.Scan(&tr.ID, &tr.RequestID, &tr.MerchantID, &tr.TerminalSN, &tr.Status, &requestedRaw, &handledByNull, &handledAtRaw, &notesNull, &tr.BusinessName, &tr.OwnerName, &tr.DeviceName); err != nil {
			log.Printf("GetTerminalRequests: scan: %v", err)
			continue
		}

		if len(requestedRaw) > 0 {
			if t, err := time.Parse(time.RFC3339, string(requestedRaw)); err == nil {
				tr.RequestedAt = t
			} else if t, err := time.ParseInLocation("2006-01-02 15:04:05", string(requestedRaw), time.Local); err == nil {
				tr.RequestedAt = t
			}
		}
		if handledByNull.Valid {
			s := handledByNull.String
			tr.HandledBy = &s
		}
		if len(handledAtRaw) > 0 {
			if t, err := time.Parse(time.RFC3339, string(handledAtRaw)); err == nil {
				tr.HandledAt = &t
			} else if t, err := time.ParseInLocation("2006-01-02 15:04:05", string(handledAtRaw), time.Local); err == nil {
				tr.HandledAt = &t
			}
		}
		if notesNull.Valid {
			s := notesNull.String
			tr.Notes = &s
		}
		requests = append(requests, tr)
	}

	totalPages := (total + limit - 1) / limit
	return TerminalRequestsResult{Requests: requests, TotalItems: total, CurrentPage: page, TotalPages: totalPages}, nil
}

// ApproveTerminalRequest grants a merchant's hardware request by assigning them an available terminal.
func (s *Service) ApproveTerminalRequest(requestID, adminUserID string, payload ApproveTerminalRequestPayload) error {
	store := s.repo.store
	tx, err := store.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var merchantID string
	var terminalSN *string
	var currentStatus string
	if err := tx.QueryRow(`SELECT merchant_id, terminal_sn, status FROM terminal_requests WHERE request_id=?`, requestID).Scan(&merchantID, &terminalSN, &currentStatus); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("terminal request not found")
		}
		return fmt.Errorf("query request: %w", err)
	}
	if currentStatus != "pending" {
		return errors.New("only pending requests can be approved")
	}

	assignSN := payload.AssignTerminalSN
	if assignSN == "" && terminalSN != nil {
		assignSN = *terminalSN
	}
	if assignSN == "" {
		return errors.New("a terminal must be assigned to approve this request")
	}

	var existingMerchantID *string
	if err := tx.QueryRow(`SELECT merchant_id FROM terminals WHERE terminal_sn=?`, assignSN).Scan(&existingMerchantID); err == nil {
		if existingMerchantID != nil && *existingMerchantID != merchantID {
			return errors.New("terminal is already assigned to another merchant")
		}
	}

	if _, err = tx.Exec(`UPDATE terminal_requests SET status='approved', handled_by=?, handled_at=CURRENT_TIMESTAMP WHERE request_id=?`, adminUserID, requestID); err != nil {
		return fmt.Errorf("update request: %w", err)
	}

	var businessAddress, city string
	if err := tx.QueryRow(`SELECT business_address, city FROM merchants WHERE merchant_id=?`, merchantID).Scan(&businessAddress, &city); err == nil {
		loc := businessAddress
		if city != "" {
			loc = businessAddress + ", " + city
		}
		if _, err = tx.Exec(`UPDATE terminals SET merchant_id=?, location_details=?, status='active' WHERE terminal_sn=?`, merchantID, loc, assignSN); err != nil {
			return fmt.Errorf("assign terminal: %w", err)
		}
	}

	return tx.Commit()
}

// RejectTerminalRequest denies a merchant's hardware application with a specified reason.
func (s *Service) RejectTerminalRequest(requestID, adminUserID string, payload RejectTerminalRequestPayload) error {
	store := s.repo.store
	var currentStatus string
	if err := store.QueryRow(`SELECT status FROM terminal_requests WHERE request_id=?`, requestID).Scan(&currentStatus); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("terminal request not found")
		}
		return fmt.Errorf("query request: %w", err)
	}
	if currentStatus != "pending" {
		return errors.New("only pending requests can be rejected")
	}

	reason := payload.Reason
	if reason == "" {
		reason = "Rejected by admin"
	}

	_, err := store.Exec(`UPDATE terminal_requests SET status='rejected', handled_by=?, handled_at=CURRENT_TIMESTAMP, notes=? WHERE request_id=?`, adminUserID, reason, requestID)
	return err
}

// ---------------------------------------------------------------------------
// Cards
// ---------------------------------------------------------------------------

// GetCardInventory retrieves all smart cards and aggregates system-wide status statistics.
func (s *Service) GetCardInventory() (CardInventoryResult, error) {
	store := s.repo.store
	var stats AdminCardInventoryStats
	store.QueryRow("SELECT COUNT(*) FROM cards").Scan(&stats.Total)
	store.QueryRow("SELECT COUNT(*) FROM cards WHERE status='Active'").Scan(&stats.Active)
	store.QueryRow("SELECT COUNT(*) FROM cards WHERE status='Inactive'").Scan(&stats.Inactive)
	store.QueryRow("SELECT COUNT(*) FROM cards WHERE status='Blocked'").Scan(&stats.Blocked)
	store.QueryRow("SELECT COUNT(*) FROM cards WHERE status='Lost'").Scan(&stats.Lost)

	rows, err := store.Query(`SELECT user_id, card_uid, card_number, card_type, balance, status, expiry_date, created_at FROM cards ORDER BY created_at DESC`)
	if err != nil {
		return CardInventoryResult{}, fmt.Errorf("query cards: %w", err)
	}
	defer rows.Close()

	var cards []AdminCard
	for rows.Next() {
		var c AdminCard
		if err := rows.Scan(&c.UserID, &c.CardUID, &c.CardNumber, &c.CardType, &c.Balance, &c.Status, &c.ExpiryDate, &c.CreatedAt); err != nil {
			log.Printf("GetCardInventory: scan: %v", err)
			continue
		}
		cards = append(cards, c)
	}
	return CardInventoryResult{Stats: stats, Cards: cards}, nil
}

// BlockCard permanently disables a smart card by its unique identifier.
func (s *Service) BlockCard(cardID string) (bool, error) {
	result, err := s.repo.store.Exec(`UPDATE cards SET status='Blocked' WHERE card_number=? OR card_uid=?`, cardID, cardID)
	if err != nil {
		return false, fmt.Errorf("block card: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// AddCard registers a new smart card into the system with an initial unlinked state.
func (s *Service) AddCard(req structs.CardData) error {
	if req.Balance.IsNegative() {
		return errors.New("initial amount cannot be negative")
	}

	cardUID := strings.TrimSpace(req.CardUID)
	exists, err := s.repo.CardUIDExist(cardUID)
	if err != nil {
		return fmt.Errorf("verify card UID: %w", err)
	}
	if exists {
		return errors.New("this Card UID is already registered in the system")
	}

	cardNumber := s.repo.GenerateCardNumber()
	expiryDate := time.Now().AddDate(2, 0, 0).Format("2006-01-02")

	_, err = s.repo.store.Exec(`INSERT INTO cards (card_uid, card_number, card_type, balance, expiry_date, status) VALUES (?, ?, 'regular', ?, ?, 'inactive')`, cardUID, cardNumber, req.Balance, expiryDate)
	return err
}

// DeactivateCard blocks an active smart card linked to a specific user.
func (s *Service) DeactivateCard(cardNumber, cardHolder, cardType string) (bool, error) {
	result, err := s.repo.store.Exec(`UPDATE cards SET status='Blocked' WHERE card_number=? AND user_id=? AND card_type=? AND status='Active'`, cardNumber, cardHolder, cardType)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// DeleteCard permanently removes a smart card record from the database.
func (s *Service) DeleteCard(cardNumber string) (bool, error) {
	result, err := s.repo.store.Exec("DELETE FROM cards WHERE card_number=?", cardNumber)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// ---------------------------------------------------------------------------
// Transactions
// ---------------------------------------------------------------------------

// GetAllTransactions retrieves a unified list of financial and audit events across the platform.
func (s *Service) GetAllTransactions() ([]TxnRow, error) {
	const q = `
		SELECT transaction_id, terminal_id, date, time, transaction_type, amount, status, description, business_name, merchant_id, points_earned, card_number, customer_name, service_fee
		FROM (
			SELECT t.transaction_id, COALESCE(t.terminal_id,'') AS terminal_id, DATE(t.created_at) as date, TIME(t.created_at) as time,
			       COALESCE(t.transaction_type,'') AS transaction_type, COALESCE(t.amount,0.00) AS amount, COALESCE(t.status,'') AS status,
			       COALESCE(t.description,'') AS description, COALESCE(m.business_name,'') AS business_name, COALESCE(m.merchant_id,'') AS merchant_id,
			       COALESCE(t.points_earned,0) AS points_earned, COALESCE(c.card_number,'') AS card_number,
			       COALESCE(u.name,'Unknown Customer') as customer_name, COALESCE(t.service_fee,0) AS service_fee, t.created_at
			FROM transactions t
			LEFT JOIN cards c ON t.card_number = c.card_number
			LEFT JOIN users u ON t.user_id = u.user_id
			LEFT JOIN merchants m ON t.merchant_id = m.merchant_id
			UNION ALL
			SELECT CONCAT('LOG-', ual.id), '', DATE(ual.created_at), TIME(ual.created_at),
			       ual.activity_type, 0.00, ual.status, COALESCE(ual.description,''),
			       COALESCE(m.business_name,''), COALESCE(m.merchant_id,''), 0, '',
			       COALESCE(u.name,'Unknown Customer'), 0.00, ual.created_at
			FROM user_activity_logs ual
			LEFT JOIN users u ON ual.user_id = u.user_id
			LEFT JOIN merchants m ON ual.user_id = m.user_id
		) AS combined ORDER BY created_at DESC`

	rows, err := s.repo.store.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var txns []TxnRow
	for rows.Next() {
		var t TxnRow
		var cardNumber *string
		if err := rows.Scan(&t.TransactionID, &t.TerminalID, &t.Date, &t.Time, &t.Type, &t.Amount, &t.Status, &t.Description, &t.MerchantName, &t.MerchantID, &t.PointsEarned, &cardNumber, &t.CustomerName, &t.ServiceFee); err != nil {
			log.Printf("GetAllTransactions: scan: %v", err)
			continue
		}

		if t.MerchantName == "" {
			t.MerchantName = "Transaction"
		}
		if t.MerchantID == "" {
			t.MerchantID = "N/A"
		}
		if t.TerminalID != "" {
			t.Source = "Terminal: " + t.TerminalID
		} else {
			t.Source = "Online/System"
		}

		cn := ""
		if cardNumber != nil {
			cn = *cardNumber
		}
		t.CardNumber = func() string {
			if cn == "" {
				return "N/A"
			}
			return cn
		}()

		merchantStr := t.MerchantName
		if t.TerminalID != "" && t.TerminalID != "N/A" {
			merchantStr += " (Terminal: " + t.TerminalID + ")"
		}
		cardStr := t.CustomerName
		if cn != "" && len(cn) >= 4 {
			cardStr += " (**** " + cn[len(cn)-4:] + ")"
		}

		if strings.ToLower(t.Type) == "payment" {
			t.Sender = cardStr
			t.Receiver = merchantStr
		} else {
			t.Sender = merchantStr
			t.Receiver = cardStr
		}
		txns = append(txns, t)
	}
	return txns, nil
}

// GetXenditTransactions fetches and formats payout events from the external Xendit gateway.
func (s *Service) GetXenditTransactions() ([]XenditTxnRow, error) {
	xenditTx, err := xenditclient.GetAllTransactions()
	if err != nil {
		return nil, fmt.Errorf("xendit API: %w", err)
	}

	loc := time.FixedZone("PHT", 8*60*60)
	var txns []XenditTxnRow

	for _, tx := range xenditTx.Data {
		updatedLoc := tx.Updated.In(loc)
		desc := string(tx.ChannelCategory)
		if tx.ChannelCode.Get() != nil {
			desc += " - " + *tx.ChannelCode.Get()
		}
		settlementStatus := "N/A"
		if tx.SettlementStatus.Get() != nil {
			settlementStatus = *tx.SettlementStatus.Get()
		}
		settlementTime := "N/A"
		if tx.EstimatedSettlementTime.Get() != nil {
			settlementTime = tx.EstimatedSettlementTime.Get().In(loc).Format("2006-01-02 15:04:05")
		}

		txns = append(txns, XenditTxnRow{
			TransactionID:    tx.ReferenceId,
			Date:             updatedLoc.Format("2006-01-02"),
			Time:             updatedLoc.Format("15:04:05"),
			Description:      "Xendit: " + desc,
			Type:             "External",
			Amount:           decimal.NewFromFloat(float64(tx.Amount)),
			Status:           string(tx.Status),
			SettlementStatus: settlementStatus,
			SettlementTime:   settlementTime,
		})
	}

	// Sort descending by date+time
	for i := 0; i < len(txns)-1; i++ {
		for j := i + 1; j < len(txns); j++ {
			if txns[i].Date+" "+txns[i].Time < txns[j].Date+" "+txns[j].Time {
				txns[i], txns[j] = txns[j], txns[i]
			}
		}
	}
	return txns, nil
}

// ---------------------------------------------------------------------------
// Terminal simulation
// ---------------------------------------------------------------------------

// GetSimMerchants retrieves a lightweight list of merchants for the transaction simulator.
func (s *Service) GetSimMerchants() []SimMerchant {
	rows, err := s.repo.store.Query("SELECT merchant_id, business_name FROM merchants")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var merchants []SimMerchant
	for rows.Next() {
		var m SimMerchant
		if err := rows.Scan(&m.ID, &m.Name); err == nil {
			merchants = append(merchants, m)
		}
	}
	return merchants
}

// SimTransaction executes a mocked payment or refund for testing and demonstration purposes.
func (s *Service) SimTransaction(req SimRequest) (decimal.Decimal, error) {
	store := s.repo.store

	var balance decimal.Decimal
	var status string
	var userID *string
	if err := store.QueryRow(`SELECT balance, status, user_id FROM cards WHERE card_number=?`, req.CardNumber).Scan(&balance, &status, &userID); err != nil {
		if err == sql.ErrNoRows {
			return decimal.Zero, errors.New("card not found")
		}
		return decimal.Zero, fmt.Errorf("query card: %w", err)
	}
	if status != "active" {
		return decimal.Zero, errors.New("card is not active")
	}
	if userID == nil || *userID == "" {
		return decimal.Zero, errors.New("card is not linked to any user")
	}
	if req.Type != "Refund" && balance.LessThan(req.Amount) {
		return decimal.Zero, errors.New("insufficient balance")
	}

	var commissionRate decimal.Decimal
	if err := store.QueryRow("SELECT commission_rate FROM merchants WHERE merchant_id=?", req.MerchantID).Scan(&commissionRate); err != nil {
		commissionRate = decimal.NewFromFloat(2)
	}

	serviceFee := req.Amount.Mul(commissionRate.Div(decimal.NewFromFloat(100)))
	loyaltyPoints := req.Amount.Mul(decimal.NewFromFloat(0.002))

	tx, err := store.Begin()
	if err != nil {
		return decimal.Zero, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if req.Type == "Refund" {
		_, err = tx.Exec(`UPDATE cards SET balance=balance+?, loyalty_points=loyalty_points-? WHERE card_number=?`, req.Amount, loyaltyPoints, req.CardNumber)
	} else {
		_, err = tx.Exec(`UPDATE cards SET balance=balance-?, loyalty_points=loyalty_points+? WHERE card_number=?`, req.Amount, loyaltyPoints, req.CardNumber)
	}
	if err != nil {
		return decimal.Zero, fmt.Errorf("update card balance: %w", err)
	}

	transactionID := fmt.Sprintf("TXN-SIM-%d", time.Now().UnixNano())

	var terminalID string
	if err := store.QueryRow("SELECT terminal_id FROM terminals LIMIT 1").Scan(&terminalID); err != nil {
		terminalID = "TRM-SIM-001"
	}
	var processedBy string
	if err := store.QueryRow("SELECT user_id FROM users LIMIT 1").Scan(&processedBy); err != nil {
		processedBy = "USR-SIM-001"
	}

	dbType := "payment"
	if req.Type == "Refund" {
		dbType = "refund"
	}

	var uid string
	_ = tx.QueryRow("SELECT user_id FROM cards WHERE card_number=?", req.CardNumber).Scan(&uid)

	_, err = tx.Exec(`INSERT INTO transactions (transaction_id, card_number, user_id, merchant_id, terminal_id, transaction_type, amount, service_fee, processed_by, points_earned) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		transactionID, req.CardNumber, uid, req.MerchantID, terminalID, dbType, req.Amount, serviceFee, processedBy, loyaltyPoints)
	if err != nil {
		return decimal.Zero, fmt.Errorf("insert transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return decimal.Zero, fmt.Errorf("commit: %w", err)
	}
	return serviceFee, nil
}