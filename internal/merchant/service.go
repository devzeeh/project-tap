package merchant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
	"project-tap/internal/pkg/storage"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

// Service implements the business logic for merchant operations.
type Service struct {
	repo          *Repository
	store         storage.Service // object storage
	payoutGateway PayoutGateway
}

// NewService creates a new merchant Service with the required repository and external integrations.
func NewService(repo *Repository, store storage.Service, payoutGateway PayoutGateway) *Service {
	return &Service{repo: repo, store: store, payoutGateway: payoutGateway}
}

var (
	ErrInvalidBankDetails = errors.New("invalid bank details")
	ErrUnsupportedBank    = errors.New("unsupported bank")
	ErrMerchantNotFound   = errors.New("merchant not found")
)

var (
	ErrFileTooLarge    = errors.New("file too large")
	ErrInvalidFileType = errors.New("invalid file type")
	ErrStorageNotReady = errors.New("storage not configured")
)

var docTypeColumnMap = map[string]string{
	"BIR Certificate": "bir_document",
	"Valid ID":        "valid_id",
	"Bank Document":   "bank_document",
	// default: "business_document"
}

var validUploadExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".pdf":  true,
}

// DocumentUpload represents the data and metadata for a file being uploaded by a merchant.
type DocumentUpload struct {
	Username    string
	DocType     string
	File        multipart.File
	Filename    string
	Size        int64
	ContentType string
}

// Merchant account services

// GetMerchantSummary retrieves the merchant's core profile, bank details, and document statuses.
func (s *Service) GetMerchantSummary(ctx context.Context, username string) (*AccountSummary, error) {
	merchant, err := s.repo.GetMerchantAccountData(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get account summary: %w", err)
	}

	summary := &AccountSummary{
		MerchantID:     merchant.merchantID,
		AccountStatus:  merchant.accountStatus,
		DocumentStatus: merchant.docStatus,
		AccountMessage: merchant.docMessage,
		MemberSince:    merchant.createdAtStr,
		BusinessDetails: BusinessDetails{
			BusinessName:    merchant.businessName,
			BusinessType:    merchant.businessType,
			BusinessEmail:   merchant.businessEmail,
			BusinessPhone:   merchant.businessPhone,
			BusinessAddress: merchant.businessAddress,
			City:            merchant.city,
			PostalCode:      merchant.postalCode,
		},
		BusinessBankDetails: BusinessBankDetails{
			AccountHolderName: merchant.accName,
			BankName:          merchant.bankName,
			AccountNumber:     maskedAccount(merchant.accNumber),
		},
	}
	return summary, nil
}

// maskedAccount obscures all but the last 4 digits of a bank account number for security.
func maskedAccount(accountNumber string) string {
	if len(accountNumber) > 4 {
		return "**** **** **** " + accountNumber[len(accountNumber)-4:]
	}
	return accountNumber
}

// buildDocuments assembles the list of submitted business documents,
// skipping any that haven't been uploaded.
func buildDocuments(m *MerchantDetails) []BusinessDocument {
	documents := []BusinessDocument{}

	// Business Registration (Business Permit)
	if m.businessDoc != "" {
		documents = append(documents, BusinessDocument{
			DocumentType:      "DTI/SEC Registration",
			Status:            m.docStatus,
			Message:           m.docMessage,
			BusinessStructure: m.businessStructure,
			DocumentURL:       m.businessDoc,
		})
	}

	// Tax Registration (BIR)
	if m.birDoc != "" {
		documents = append(documents, BusinessDocument{
			DocumentType: "BIR Certificate",
			Status:       m.docStatus,
			Message:      m.docMessage,
			DocumentURL:  m.birDoc,
		})
	}

	// Valid Government ID (PhilHealth, SSS, Pag-IBIG)
	if m.validID != "" {
		documents = append(documents, BusinessDocument{
			DocumentType: "Valid Government ID",
			Status:       m.docStatus,
			Message:      m.docMessage,
			DocumentURL:  m.validID,
		})
	}

	if m.bankDoc != "" {
		documents = append(documents, BusinessDocument{
			DocumentType: "Bank Document",
			Status:       m.docStatus,
			Message:      m.docMessage,
			DocumentURL:  m.bankDoc,
		})
	}
	return documents
}

// UpdateBankDetails validates and updates the merchant's settlement bank information.
func (s *Service) UpdateBankDetails(ctx context.Context, username string, req BusinessBankDetails) error {
	req.BankName = strings.TrimSpace(req.BankName)
	req.AccountHolderName = strings.TrimSpace(req.AccountHolderName)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)

	if req.BankName == "" || req.AccountHolderName == "" || req.AccountNumber == "" {
		return ErrInvalidBankDetails
	}

	if _, ok := channelCodeMap[req.BankName]; !ok {
		return ErrUnsupportedBank
	}

	merchantID, userID, existingAccNumber, err := s.repo.GetMerchantBankDetails(ctx, username)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMerchantNotFound, err)
	}

	// Prevent overwriting with a masked account number (e.g. "**** **** **** 1234")
	if existingAccNumber != nil && strings.Contains(req.AccountNumber, "****") {
		req.AccountNumber = *existingAccNumber
	}

	if err := s.repo.UpdateBankDetails(ctx, merchantID, req); err != nil {
		return err
	}

	if err := s.repo.InsertBankUpdateLog(ctx, userID, "profile_update", "in_app", "completed",
		"Settlement bank details were updated by the merchant."); err != nil {
		log.Printf("UpdateBankDetails: failed to log activity for user %s: %v", userID, err)
	}

	return nil
}

// Upload documents

// set the max file size to 5MB
const maxUploadSize = 5 << 20

// UploadDocument validates the file size and type, then uploads it to the storage service and updates the database.
func (s *Service) UploadDocument(ctx context.Context, up DocumentUpload) error {
	if s.store == nil {
		return ErrStorageNotReady
	}
	if up.Size > maxUploadSize {
		return ErrFileTooLarge
	}

	ext := strings.ToLower(filepath.Ext(up.Filename))
	if !validUploadExts[ext] {
		return ErrInvalidFileType
	}

	exists, err := s.repo.MerchantExists(ctx, up.Username)
	if err != nil {
		return fmt.Errorf("upload document: %w", err)
	}
	if !exists {
		return ErrMerchantNotFound
	}

	column, ok := docTypeColumnMap[up.DocType]
	if !ok {
		column = "business_document"
	}

	if _, err := up.File.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek uploaded file: %w", err)
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dbPath, err := s.store.UploadFile(ctx, up.File, filename, up.ContentType)
	if err != nil {
		return fmt.Errorf("upload file to storage: %w", err)
	}

	s.deleteOldDocument(ctx, up.Username, column)

	if err := s.repo.UpdateDocumentPath(ctx, up.Username, column, dbPath); err != nil {
		return fmt.Errorf("upload document: %w", err)
	}

	return nil
}

// deleteOldDocument removes the previously stored file, if any. Best-effort:
// failures are logged but never block the upload of the new document.
func (s *Service) deleteOldDocument(ctx context.Context, username, column string) {
	oldPath, err := s.repo.GetDocumentPath(ctx, username, column)
	if err != nil || oldPath == nil || *oldPath == "" {
		return
	}

	oldFile := strings.TrimPrefix(*oldPath, "/")

	switch {
	case strings.HasPrefix(oldFile, "storage/documents/"):
		key := strings.TrimPrefix(oldFile, "storage/")
		if err := s.store.DeleteFile(ctx, key); err != nil {
			log.Printf("deleteOldDocument: failed to remove %s from R2: %v", key, err)
		}
	case !strings.HasPrefix(oldFile, "http"):
		if err := os.Remove(oldFile); err != nil {
			log.Printf("deleteOldDocument: failed to remove local file %s: %v", oldFile, err)
		}
	}
}

// GetDashboard aggregates various data points to populate the merchant dashboard, including revenue, transactions, and account status.
func (s *Service) GetDashboard(ctx context.Context, username string) (*MerchantSummary, error) {
	core, err := s.repo.GetMerchantByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}
	merchantID := core.MerchantID

	var (
		accountStatus, accountRole string
		totalTransactions          int
		revenue                    *RevenueSummary
		refunds                    decimal.Decimal
		recentTxns                 []MerchantTransaction
		incomeStats                *IncomeStat
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() (err error) {
		accountStatus, accountRole, err = s.repo.GetMerchantAccountInfo(gctx, merchantID)
		return err
	})
	g.Go(func() (err error) {
		totalTransactions, err = s.repo.CountMerchantTransactions(gctx, merchantID)
		return err
	})
	g.Go(func() (err error) {
		revenue, err = s.repo.GetRevenueSummary(gctx, merchantID)
		return err
	})
	g.Go(func() (err error) {
		refunds, err = s.repo.GetTotalRefunds(gctx, merchantID)
		return err
	})
	g.Go(func() (err error) {
		recentTxns, err = s.repo.GetRecentTransactions(gctx, merchantID, 10)
		return err
	})
	g.Go(func() error {
		// Non-critical: dashboard should still render if income stats fail.
		stats, err := s.repo.GetIncomeStats(gctx, merchantID)
		if err != nil {
			log.Printf("GetDashboard: failed to fetch income stats for merchant %s: %v", merchantID, err)
			return nil
		}
		incomeStats = stats
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("get dashboard (merchant %q): %w", merchantID, err)
	}

	summary := &MerchantSummary{
		Username:           username,
		MerchantID:         merchantID,
		AccountRole:        accountRole,
		AccountStatus:      accountStatus,
		TotalTransactions:  totalTransactions,
		GrossRevenue:       revenue.TotalRevenue,
		TotalRefunds:       refunds,
		NetRevenue:         revenue.TotalRevenue.Sub(refunds),
		TotalServiceFee:    revenue.TotalServiceFee,
		TotalIncome:        revenue.TotalIncome,
		SettlementBank:     core.SettlementBank,
		SettlementAccount:  core.SettlementAccount,
		RecentTransactions: recentTxns,
	}

	if incomeStats != nil {
		summary.AvailableBalance = incomeStats.AvailableBalance
		summary.MonthlyNetIncome = incomeStats.MonthlyNetIncome
	}

	return summary, nil
}

// GetIncome retrieves the merchant's net income statistics and transaction history.
func (s *Service) GetIncome(ctx context.Context, username string) (*IncomeResponse, error) {
	merchantID, err := s.repo.GetMerchantIDByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get income: %w", err)
	}

	var (
		stats   *IncomeStat
		history []IncomeHistory
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() (err error) {
		stats, err = s.repo.GetIncomeStats(gctx, merchantID)
		return err
	})
	g.Go(func() (err error) {
		history, err = s.repo.GetIncomeHistory(gctx, merchantID, 15)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("get income (merchant %q): %w", merchantID, err)
	}

	// Ledger balance: net revenue minus everything already withdrawn.
	stats.AvailableBalance = stats.NetRevenue.Sub(stats.TotalWithdrawn)

	return &IncomeResponse{
		Stats:   *stats,
		History: history,
	}, nil
}

// merchant/service.go

var (
	ErrInvalidAmount       = errors.New("invalid withdrawal amount")
	ErrBelowMinimum        = errors.New("below minimum withdrawal")
	ErrBankNotConfigured   = errors.New("settlement bank not configured")
	ErrDailyLimitExceeded  = errors.New("daily withdrawal limit exceeded")
	ErrInsufficientBalance = errors.New("insufficient available balance")
	ErrUnmappedBank        = errors.New("bank not supported for payout")
)

const (
	minWithdrawal    = 500.00
	dailyLimit       = 500000.00
	payoutServiceFee = 15.00
)

// PayoutGateway abstracts the payment-provider call so the service can be
// tested without hitting Xendit, and so the client is built once at startup.
type PayoutGateway interface {
	CreatePayout(ctx context.Context, txnID, channelCode, accountNumber, accountHolderName string, amount float64, description string) error
}

// WithdrawResult represents the outcome of a successful withdrawal request.
type WithdrawResult struct {
	TransactionID string
	Amount        decimal.Decimal
	Status        string
}

// Withdraw handles the business rules for a merchant requesting a payout to their settlement bank.
func (s *Service) Withdraw(ctx context.Context, username string, amount decimal.Decimal) (*WithdrawResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidAmount
	}
	if amount.LessThan(decimal.NewFromFloat(minWithdrawal)) {
		return nil, ErrBelowMinimum
	}

	bank, err := s.repo.GetMerchantBankInfo(ctx, username)
	if err != nil {
		return nil, err // already wrapped as ErrMerchantNotFound or a DB error
	}

	if bank.SettlementBank == nil || bank.SettlementAcct == nil || bank.SettlementName == nil {
		return nil, ErrBankNotConfigured
	}

	bankName := strings.TrimSpace(*bank.SettlementBank)
	channelCode, ok := channelCodeMap[bankName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnmappedBank, bankName)
	}

	stats, err := s.repo.GetIncomeStats(ctx, bank.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	availableBalance := stats.NetRevenue.Sub(stats.TotalWithdrawn)

	if amount.GreaterThan(availableBalance) {
		return nil, fmt.Errorf("%w: available %s", ErrInsufficientBalance, availableBalance.StringFixed(2))
	}

	dailyWithdrawn, err := s.repo.GetDailyWithdrawnAmount(ctx, bank.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	limit := decimal.NewFromFloat(dailyLimit)
	if dailyWithdrawn.Add(amount).GreaterThan(limit) {
		remaining := limit.Sub(dailyWithdrawn)
		return nil, fmt.Errorf("%w: %s remaining today", ErrDailyLimitExceeded, remaining.StringFixed(2))
	}

	txnID := fmt.Sprintf("TXN-WD-%d", time.Now().UnixNano())
	serviceFee := decimal.NewFromFloat(payoutServiceFee)
	accountNum := *bank.SettlementAcct
	last4 := accountNum
	if len(accountNum) > 4 {
		last4 = accountNum[len(accountNum)-4:]
	}
	description := fmt.Sprintf("Withdrawal to %s ending in %s", *bank.SettlementBank, last4)

	// Record the pending transaction BEFORE calling the payment gateway,
	// so there's always a ledger entry even if the gateway call fails or
	// this process crashes mid-call.
	if err := s.repo.InsertWithdrawalTransaction(ctx, txnID, bank.MerchantID, bank.UserID, description, amount, serviceFee, "pending"); err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}

	payoutAmount := amount.Sub(serviceFee).InexactFloat64()
	if err := s.payoutGateway.CreatePayout(ctx, txnID, channelCode, *bank.SettlementAcct, *bank.SettlementName, payoutAmount, description); err != nil {
		log.Printf("Withdraw: payout gateway failed for txn %s: %v", txnID, err)
		if uerr := s.repo.UpdateWithdrawalStatus(ctx, txnID, "failed", "pending"); uerr != nil {
			log.Printf("Withdraw: failed to mark txn %s as failed after gateway error: %v", txnID, uerr)
		}
		return nil, fmt.Errorf("payment gateway rejected withdrawal: %w", err)
	}

	return &WithdrawResult{TransactionID: txnID, Amount: amount, Status: "pending"}, nil
}

// RequestTerminal creates a new terminal allocation request for the merchant.
func (s *Service) RequestTerminal(ctx context.Context, username, terminalSN, notes string) (string, error) {
	merchantID, err := s.repo.GetMerchantIDByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	requestID := fmt.Sprintf("TRQ-%d", time.Now().UnixNano()/1_000_000)
	if err := s.repo.InsertTerminalRequest(ctx, requestID, merchantID, terminalSN, notes); err != nil {
		return "", err
	}
	return requestID, nil
}

// GetTransactions retrieves a filtered list of the merchant's transactions.
func (s *Service) GetTransactions(ctx context.Context, username string, filter TransactionFilter) ([]MerchantTransaction, error) {
	merchantID, err := s.repo.GetMerchantIDByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return s.repo.GetTransactions(ctx, merchantID, filter)
}

// ProcessDisbursementWebhook handles status updates from the payment gateway regarding payouts.
func (s *Service) ProcessDisbursementWebhook(ctx context.Context, externalID, event, failureCode string) error {
	switch event {
	case "payout.succeeded":
		if err := s.repo.UpdateWithdrawalStatus(ctx, externalID, "completed", "pending"); err != nil {
			return fmt.Errorf("process disbursement webhook (succeeded, txn %q): %w", externalID, err)
		}
	case "payout.failed":
		if err := s.repo.UpdateWithdrawalStatus(ctx, externalID, "failed", "pending"); err != nil {
			return fmt.Errorf("process disbursement webhook (failed, txn %q): %w", externalID, err)
		}
		log.Printf("Disbursement failed for transaction %s. Reason: %s", externalID, failureCode)
	}
	return nil
}
