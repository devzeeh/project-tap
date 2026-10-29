package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"project-tap/internal/pkg/account"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

// EmailSender abstracts outgoing email so the service is testable without a
// live SMTP server, and so the client is configured once, not per-request.
type EmailSender interface {
	SendEmailChangeVerification(to, newEmail, name, verifyURL string) error
}

type InvoiceGateway interface {
	CreateInvoice(ctx context.Context, req CreateInvoiceParams) (invoiceURL string, err error)
}

type Service struct {
	repo    *Repository
	mailer  EmailSender
	invoice InvoiceGateway
}

func NewService(repo *Repository, mailer EmailSender, invoice InvoiceGateway) *Service {
	return &Service{repo: repo, mailer: mailer, invoice: invoice}
}

var validCardStatuses = map[string]bool{
	"active":   true,
	"inactive": true,
	"blocked":  true,
	"lost":     true,
}

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidCardStatus     = errors.New("invalid card status")
	ErrMissingPasswordField  = errors.New("missing password field")
	ErrPasswordMismatch      = errors.New("passwords do not match")
	ErrNoFieldsToUpdate      = errors.New("no fields to update")
	ErrSamePassword          = errors.New("new password must differ from current")
	ErrPasswordLookupFailed  = errors.New("failed to look up password hash")
	ErrIncorrectPassword     = errors.New("incorrect password")
	ErrCardNotFound          = errors.New("card not found")
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrTopUpTooLow           = errors.New("top-up amount too low")
	ErrTopUpTooHigh          = errors.New("top-up amount too high")
	ErrTopUpAlreadyCompleted   = errors.New("top-up already completed")
	ErrTopUpNotFound           = errors.New("top-up not found")
	ErrInvalidToken            = errors.New("invalid or expired token")
	ErrNoPendingEmail          = errors.New("no pending email to verify")
	ErrUnsupportedPaymentMethod = errors.New("unsupported payment method")
)

const (
	cardReplacementFee = 150.0
	convenienceFee     = 15.00
	topUpMinAmount     = 50.00
	topUpMaxAmount     = 2000.00
	invoiceDuration    = 15 * 60 // seconds
)

type CreateInvoiceParams struct {
	ExternalID     string
	TotalAmount    float64
	ItemAmount     float64
	FeeAmount      float64
	PayerEmail     string
	Description    string
	PaymentMethods []string
	SuccessURL     string
	FailureURL     string
}

type TxnResponse struct {
	TransactionID string          `json:"transaction_id"`
	TerminalID    string          `json:"terminal_id"`
	Date          string          `json:"date"`
	Time          string          `json:"time"`
	Description   string          `json:"description"`
	Type          string          `json:"type"`
	Amount        float64         `json:"amount"`
	Status        string          `json:"status"`
	MerchantName  string          `json:"merchant_name"`
	MerchantID    string          `json:"merchant_id"`
	ServiceFee    float64         `json:"service_fee"`
	PointsEarned  decimal.Decimal `json:"points_earned"`
	Sender        string          `json:"sender"`
	Receiver      string          `json:"receiver"`
}

// Dashboard services

func (s *Service) GetDashboardUser(ctx context.Context, username string) (*DashboardUser, error) {
	var (
		userCard *userCardRow
		txnRows  []Transaction
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() (err error) {
		userCard, err = s.repo.GetUserByusername(gctx, username)
		return err
	})
	g.Go(func() error {
		// Non-fatal: dashboard should still render if recent transactions fail.
		rows, err := s.repo.GetRecentTransaction(gctx, username)
		if err != nil {
			log.Printf("GetDashboardUser: failed to fetch recent transactions for %s: %v", username, err)
			return nil
		}
		txnRows = rows
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("get dashboard user: %w", err)
	}

	dashboardUser := &DashboardUser{
		ID:                 userCard.id,
		UserID:             username,
		Username:           userCard.username,
		Name:               userCard.fullName,
		Email:              userCard.email,
		PendingEmail:       userCard.pendingEmail,
		Phone:              userCard.phone,
		Initials:           buildInitials(userCard.fullName),
		Balance:            userCard.balance,
		LoyaltyPoints:      userCard.loyaltyPoints,
		AccountType:        userCard.userType,
		CardNumber:         userCard.cardNumber,
		CardExpiry:         formatCardExpiry(userCard.expiryDate),
		CardStatus:         userCard.cardStatus,
		UserStatus:         userCard.userStatus,
		RecentTransactions: buildTransactions(txnRows),
	}

	return dashboardUser, nil
}

// formatDate converts a DB timestamp string into a display date like "Jan 5, 2026".
// Falls back to a raw date substring, then the original string, if parsing fails.
func formatDate(dbTime string) string {
	if t, err := time.Parse("2006-01-02 15:04:05", dbTime); err == nil {
		return t.Format("Jan _2, 2006")
	}
	if t, err := time.Parse(time.RFC3339, dbTime); err == nil {
		return t.Format("Jan _2, 2006")
	}
	if len(dbTime) >= 10 {
		return dbTime[:10]
	}
	return dbTime
}

// formatTime converts a DB timestamp string into a display time like "03:04 PM".
// Returns an empty string if the value can't be parsed or is too short to contain a time.
func formatTime(dbTime string) string {
	if t, err := time.Parse("2006-01-02 15:04:05", dbTime); err == nil {
		return t.Format("03:04 PM")
	}
	if t, err := time.Parse(time.RFC3339, dbTime); err == nil {
		return t.Format("03:04 PM")
	}
	if len(dbTime) > 10 {
		return dbTime[11:16]
	}
	return ""
}

// buildInitials derives up to 2 uppercase initials from a full name,
// falling back to "U" if the name is empty.
func buildInitials(fullName string) string {
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
	return strings.ToUpper(initials)
}

// formatCardExpiry converts a "2006-01-02"-style date string into "01/06".
// Returns a placeholder if the date is missing or malformed.
func formatCardExpiry(expiryDate string) string {
	if len(expiryDate) < 10 {
		return "MM/YY"
	}
	t, err := time.Parse("2006-01-02", expiryDate[:10])
	if err != nil {
		return "MM/YY"
	}
	return t.Format("01/06")
}

// buildTransactions maps raw repository rows into the Transaction view type,
// unwrapping nullable fields and applying the merchant-name fallback rule.
func buildTransactions(rows []Transaction) []Transaction {
	transactions := make([]Transaction, 0, len(rows))

	for _, r := range rows {
		t := Transaction{
			TransactionID: r.TransactionID,
			Date:          formatDate(r.CreatedAt),
			Time:          formatTime(r.CreatedAt),
			Description:     r.Description,
			Type:            r.Type,
			TransactionType: r.Type,
			Amount:          r.Amount,
			TerminalID:    r.TerminalID,
			Status:        r.Status,
			ServiceFee:    0.00,
			PointsEarned:  r.PointsEarned,
		}

		transactions = append(transactions, t)
	}

	return transactions
}

// Customer profile

// VerifyPassword checks a plaintext password against the stored hash for
// username, returning the hash on success so callers can run further checks.
func (s *Service) VerifyPassword(ctx context.Context, username, password string) (string, error) {
	hash, err := s.repo.GetPasswordHash(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", err
		}
		return "", fmt.Errorf("%w: %v", ErrPasswordLookupFailed, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrIncorrectPassword
	}

	return hash, nil
}

type ProfileUpdateResult struct {
	Message      string
	EmailChanged bool
}

func (s *Service) UpdateProfile(ctx context.Context, username string, req ProfileUpdateRequest) (*ProfileUpdateResult, error) {
	if req.FullName == "" && req.Email == "" && req.Phone == "" && req.Username == "" {
		return nil, ErrNoFieldsToUpdate
	}

	var currentEmail, currentName string
	emailChanged := false

	if req.Email != "" {
		email, name, err := s.repo.GetEmailAndName(ctx, username)
		switch {
		case err == nil:
			currentEmail, currentName = email, name
			emailChanged = req.Email != currentEmail
		case errors.Is(err, ErrUserNotFound):
			// fall through with emailChanged=false, matching original behavior
		default:
			log.Printf("UpdateProfile: failed to get current user data for %s: %v", username, err)
		}
	}

	fields := []string{}
	args := []any{}

	if req.FullName != "" {
		fields = append(fields, "name = ?")
		args = append(args, req.FullName)
		currentName = req.FullName // use updated name for email if changed
	}

	var verifyToken string
	switch {
	case req.Email != "" && !emailChanged:
		fields = append(fields, "email = ?")
		args = append(args, req.Email)
	case req.Email != "" && emailChanged:
		token, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("update profile: generate verification token: %w", err)
		}
		verifyToken = token
		fields = append(fields, "pending_email = ?", "email_verification_token = ?")
		args = append(args, req.Email, token)
	}

	if req.Phone != "" {
		fields = append(fields, "phone_number = ?")
		args = append(args, req.Phone)
	}
	if req.Username != "" {
		fields = append(fields, "username = ?")
		args = append(args, req.Username)
	}

	if len(fields) == 0 {
		return &ProfileUpdateResult{Message: "No changes to update"}, nil
	}

	if err := s.repo.UpdateProfileFields(ctx, username, fields, args); err != nil {
		return nil, err
	}

	if emailChanged && s.mailer != nil {
		go s.sendEmailChangeVerification(currentEmail, req.Email, currentName, verifyToken)
	}

	msg := "Profile updated successfully"
	if emailChanged {
		msg = "Profile updated. Please check your current email to approve the change."
	}

	return &ProfileUpdateResult{Message: msg, EmailChanged: emailChanged}, nil
}

func (s *Service) sendEmailChangeVerification(currentEmail, newEmail, name, token string) {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3001"
	}
	verifyURL := fmt.Sprintf("%s/v1/verify-email?token=%s", baseURL, token)

	if err := s.mailer.SendEmailChangeVerification(currentEmail, newEmail, name, verifyURL); err != nil {
		log.Printf("sendEmailChangeVerification: failed to send to %s: %v", currentEmail, err)
	}
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) ChangePassword(ctx context.Context, username string, req ProfileUpdateRequest) error {
	if req.CurrentPassword == "" || req.NewPassword == "" || req.ConfirmPassword == "" {
		return ErrMissingPasswordField
	}
	if req.NewPassword != req.ConfirmPassword {
		return ErrPasswordMismatch
	}

	currentHash, err := s.VerifyPassword(ctx, username, req.CurrentPassword)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.NewPassword)); err == nil {
		return ErrSamePassword
	}

	hashedPassword, err := account.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("change password: hash new password: %w", err)
	}

	return s.repo.UpdatePasswordHash(ctx, username, hashedPassword)
}

// Cards

func (s *Service) UpdateCardStatus(ctx context.Context, username, status string) error {
	if !validCardStatuses[status] {
		return ErrInvalidCardStatus
	}

	userID, err := s.repo.GetUserID(ctx, username)
	if err != nil {
		return err
	}

	return s.repo.UpdateCardStatus(ctx, status, userID)
}

type ReplacementResult struct {
	TransactionID string
}

func (s *Service) RequestCardReplacement(ctx context.Context, username string) (*ReplacementResult, error) {
	userID, err := s.repo.GetUserID(ctx, username)
	if err != nil {
		return nil, err
	}

	card, err := s.repo.GetCardBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	if card.Balance.LessThan(decimal.NewFromFloat(cardReplacementFee)) {
		return nil, ErrInsufficientBalance
	}

	txnID := fmt.Sprintf("REP-%s-%d", card.CardNumber, time.Now().Unix())

	if err := s.repo.ReplaceCard(ctx, userID, card.CardNumber, txnID, cardReplacementFee); err != nil {
		return nil, err
	}

	return &ReplacementResult{TransactionID: txnID}, nil
}

func (s *Service) CreateTopUpInvoice(ctx context.Context, username string, req TopUpRequest, baseURL string) (invoiceURL string, err error) {
	if req.Amount.LessThan(decimal.NewFromFloat(topUpMinAmount)) {
		return "", ErrTopUpTooLow
	}
	if req.Amount.GreaterThan(decimal.NewFromFloat(topUpMaxAmount)) {
		return "", ErrTopUpTooHigh
	}

	feeAmount := decimal.NewFromFloat(convenienceFee)
	totalAmount := req.Amount.Add(feeAmount)

	payer, err := s.repo.GetCardAndEmailByUsername(ctx, username)
	if err != nil {
		return "", err
	}

	allowedMethods := map[string]bool{
		"GCASH":   true,
		"PAYMAYA": true,
		"GRABPAY": true,
	}

	if req.PaymentMethod != "" && !allowedMethods[req.PaymentMethod] {
		return "", ErrUnsupportedPaymentMethod
	}

	topupID := fmt.Sprintf("TOPUP-%d", time.Now().UnixNano())

	if err := s.repo.InsertPendingTopUp(ctx, topupID, payer.CardNumber, req.Amount, feeAmount); err != nil {
		return "", err
	}

	methods := []string{"GCASH", "PAYMAYA", "GRABPAY"}
	if req.PaymentMethod != "" {
		methods = []string{req.PaymentMethod}
	}

	url, err := s.invoice.CreateInvoice(ctx, CreateInvoiceParams{
		ExternalID:     topupID,
		TotalAmount:    totalAmount.InexactFloat64(),
		ItemAmount:     req.Amount.InexactFloat64(),
		FeeAmount:      feeAmount.InexactFloat64(),
		PayerEmail:     payer.Email,
		Description:    fmt.Sprintf("Unicard Top-Up (Card: %s)", payer.CardNumber),
		PaymentMethods: methods,
		SuccessURL:     baseURL + "/u/" + username + "/dashboard",
		FailureURL:     baseURL + "/u/" + username + "/topup",
	})
	if err != nil {
		log.Printf("CreateTopUpInvoice: gateway failed for topup %s: %v", topupID, err)
		return "", fmt.Errorf("create invoice: %w", err)
	}

	return url, nil
}

func (s *Service) CompleteTopUp(ctx context.Context, rec TopUpRecord) error {
	userID, err := s.repo.GetUserIDByCardNumber(ctx, rec.CardNumber)
	if err != nil {
		return err
	}

	topupID := fmt.Sprintf("TOPUP-%d", time.Now().UnixNano())
	transactionID := fmt.Sprintf("TX-%d", time.Now().UnixNano())

	return s.repo.CompleteTopUp(ctx, rec, topupID, transactionID, userID)
}

func (s *Service) GetUserTransactions(ctx context.Context, username string) ([]TxnResponse, error) {
	rows, err := s.repo.GetUserTransactions(ctx, username)
	if err != nil {
		return nil, err
	}

	transactions := make([]TxnResponse, 0, len(rows))
	for _, r := range rows {
		t := TxnResponse{
			TransactionID: r.transactionID,
			TerminalID:    r.terminalID,
			Date:          formatDate(r.createdAt),
			Time:          formatTime(r.createdAt),
			Description:   r.description,
			Type:          r.txnType,
			Amount:        r.amount,
			Status:        r.status,
			ServiceFee:    0.00,
			PointsEarned:  r.pointsEarned,
		}

		if r.businessName != "" {
			t.MerchantName = r.businessName
		} else {
			t.MerchantName = "Transaction"
		}

		if r.merchantID != "" {
			t.MerchantID = r.merchantID
		} else {
			t.MerchantID = "N/A"
		}

		t.Sender, t.Receiver = buildSenderReceiver(t.Type, t.Description, t.TerminalID, r.cardNumber)

		transactions = append(transactions, t)
	}

	return transactions, nil
}

// buildSenderReceiver derives display strings for who paid whom, based on
// transaction direction. Payments flow card -> merchant; everything else
// (refunds, topups, etc.) flows merchant -> card.
func buildSenderReceiver(txnType, description, terminalID, cardNumber string) (sender, receiver string) {
	merchantStr := description
	if terminalID != "" && terminalID != "N/A" {
		merchantStr += " (Terminal: " + terminalID + ")"
	}

	cardStr := "My UniCard"
	if len(cardNumber) >= 4 {
		cardStr += " (**** " + cardNumber[len(cardNumber)-4:] + ")"
	}

	if strings.ToLower(txnType) == "payment" {
		return cardStr, merchantStr
	}
	return merchantStr, cardStr
}

func (s *Service) GetUsernameByCardNumber(ctx context.Context, cardNumber string) (string, error) {
	return s.repo.GetUsernameByCardNumber(ctx, cardNumber)
}

func (s *Service) ProcessTopUpWebhook(ctx context.Context, payload XenditWebhookPayload) (string, error) {
	username, _ := s.repo.GetUsernameByTopUpID(ctx, payload.ExternalID)

	switch payload.Status {
	case "PAID", "SETTLED":
		err := s.repo.CompletePendingTopUp(ctx, payload.ExternalID)
		if err != nil && !errors.Is(err, ErrTopUpAlreadyCompleted) {
			return username, err
		}
		return username, nil

	case "PENDING":
		return username, s.repo.UpdateTopUpStatus(ctx, payload.ExternalID, strings.ToLower(payload.Status))

	case "EXPIRED", "FAILED", "CANCELLED":
		if err := s.repo.UpdateTopUpStatus(ctx, payload.ExternalID, strings.ToLower(payload.Status)); err != nil {
			return username, err
		}
		if err := s.repo.DeletePendingTopUpTransaction(ctx, payload.ExternalID); err != nil {
			log.Printf("ProcessTopUpWebhook: failed to delete pending transaction for %s: %v", payload.ExternalID, err)
			// non-fatal: don't fail the webhook ack over cleanup
		}
		return username, nil
	}

	return username, nil
}


func (s *Service) VerifyEmail(ctx context.Context, token string) (username string, err error) {
	username, pendingEmail, err := s.repo.GetPendingEmailByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if pendingEmail == "" {
		return "", ErrNoPendingEmail
	}
	if err := s.repo.ConfirmEmailChange(ctx, username, pendingEmail); err != nil {
		return "", err
	}
	return username, nil
}