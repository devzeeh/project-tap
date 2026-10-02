package user

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"project-tap/internal/pkg/account"
	smtp "project-tap/internal/pkg/smtpbody"

	"github.com/shopspring/decimal"
	xendit "github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/invoice"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gomail.v2"
)

// Service encapsulates business logic for the user package.
type Service struct {
	repo *Repository
}

// NewService creates a new Service instance.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// defaultPaymentMethods defines the fallback payment channels for Xendit checkout sessions.
var defaultPaymentMethods = []string{
	"CREDIT_CARD", "GCASH", "PAYMAYA",
	"GRABPAY", "SHOPEEPAY", "7ELEVEN",
}

// IsUserAuthorized verifies that the claims user ID matches the target username.
func (s *Service) IsUserAuthorized(ctx context.Context, claimsUserID, targetUsername string, isSuperAdmin bool) bool {
	if isSuperAdmin {
		return true
	}
	if strings.EqualFold(claimsUserID, targetUsername) {
		return true
	}

	requestingUsername, err := s.repo.FindUsernameByUserIDOrID(ctx, claimsUserID)
	if err != nil {
		log.Printf("[AUTH DENIED] Failed to query username for user_id=%s: %v", claimsUserID, err)
		return false
	}

	if !strings.EqualFold(requestingUsername, targetUsername) {
		log.Printf("[AUTH DENIED] Token user %q (id: %s) does not match requested target %q", requestingUsername, claimsUserID, targetUsername)
		return false
	}

	return true
}

// GetDashboard retrieves dashboard profile and recent transactions.
func (s *Service) GetDashboard(ctx context.Context, username string) (DashboardUser, error) {
	user, err := s.repo.GetDashboardUserData(ctx, username)
	if err != nil {
		return DashboardUser{}, err
	}

	recentTxns, err := s.repo.GetRecentTransactions(ctx, username)
	if err == nil {
		user.RecentTransactions = recentTxns
	}

	return user, nil
}

// GetTransactions retrieves the full transactions list for a user.
func (s *Service) GetTransactions(ctx context.Context, username string) (TransactionsListResponse, error) {
	txns, err := s.repo.GetTransactionsList(ctx, username)
	if err != nil {
		return TransactionsListResponse{}, err
	}
	return TransactionsListResponse{
		Success:      true,
		Transactions: txns,
	}, nil
}

// UpdateCardStatus validates status and updates the card status for the target user.
func (s *Service) UpdateCardStatus(ctx context.Context, username, status string) error {
	if status != "active" && status != "inactive" && status != "blocked" && status != "lost" {
		return ErrInvalidStatus
	}

	userID, err := s.repo.GetUserIDByUsername(ctx, username)
	if err != nil {
		return ErrUserNotFound
	}

	return s.repo.UpdateCardStatus(ctx, userID, status)
}

// RequestCardReplacement verifies balance, deducts fee, and blocks the card.
func (s *Service) RequestCardReplacement(ctx context.Context, username string) error {
	userID, err := s.repo.GetUserIDByUsername(ctx, username)
	if err != nil {
		return ErrUserNotFound
	}

	balance, cardNumber, err := s.repo.GetCardDetails(ctx, userID)
	if err != nil {
		return ErrCardNotFound
	}

	if balance < CardReplacementFee {
		return ErrInsufficientBalance
	}

	return s.repo.RequestCardReplacement(ctx, userID, cardNumber, CardReplacementFee)
}

// VerifyCurrentPassword checks if the current password is valid.
func (s *Service) VerifyCurrentPassword(ctx context.Context, username, password string) error {
	hash, err := s.repo.GetPasswordHash(ctx, username)
	if err != nil {
		return ErrPasswordLookupFailed
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrIncorrectPassword
	}
	return nil
}

// ChangePassword validates passwords, checks current password, and updates hash.
func (s *Service) ChangePassword(ctx context.Context, username, currentPassword, newPassword, confirmPassword string) error {
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		return errors.New("all password fields are required")
	}
	if newPassword != confirmPassword {
		return errors.New("passwords do not match")
	}

	hash, err := s.repo.GetPasswordHash(ctx, username)
	if err != nil {
		return ErrPasswordLookupFailed
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)); err != nil {
		return ErrIncorrectPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(newPassword)); err == nil {
		return ErrSamePassword
	}

	hashedPassword, err := account.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return s.repo.UpdatePassword(ctx, username, hashedPassword)
}

// UpdateProfile updates user profile details and initiates email verification if changed.
func (s *Service) UpdateProfile(ctx context.Context, username string, req ProfileUpdateRequest) (emailChanged bool, err error) {
	fields := []string{}
	args := []any{}

	var currentEmail, currentName string
	if req.Email != "" {
		currentEmail, currentName, err = s.repo.GetEmailAndName(ctx, username)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Failed to get current user data: %v", err)
		} else if req.Email != currentEmail {
			emailChanged = true
		}
	}

	if req.FullName != "" {
		fields = append(fields, "name = ?")
		args = append(args, req.FullName)
		currentName = req.FullName
	}

	if req.Email != "" && !emailChanged {
		fields = append(fields, "email = ?")
		args = append(args, req.Email)
	} else if req.Email != "" && emailChanged {
		b := make([]byte, 16)
		rand.Read(b)
		token := hex.EncodeToString(b)

		fields = append(fields, "pending_email = ?")
		args = append(args, req.Email)
		fields = append(fields, "email_verification_token = ?")
		args = append(args, token)

		go s.sendVerificationEmail(currentEmail, req.Email, currentName, token)
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
		return false, nil
	}

	args = append(args, username)
	query := "UPDATE users SET " + strings.Join(fields, ", ") + " WHERE username = ?"

	if err := s.repo.UpdateProfile(ctx, query, args); err != nil {
		return false, err
	}

	return emailChanged, nil
}

func (s *Service) sendVerificationEmail(emailTo, emailNew, name, token string) {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := 587
	smtpEmail := os.Getenv("SMTP_EMAIL")
	smtpSender := os.Getenv("SMTP_SENDER")
	smtpPass := os.Getenv("SMTP_PASSWORD")

	if smtpHost == "" || smtpEmail == "" {
		log.Println("SMTP credentials not configured")
		return
	}

	m := gomail.NewMessage()
	m.SetHeader("From", smtpSender+" <"+smtpEmail+">")
	m.SetHeader("To", emailTo)
	m.SetHeader("Subject", "Approve Your Email Change")

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3001"
	}
	verifyURL := fmt.Sprintf("%s/v1/verify-email?token=%s", baseURL, token)
	htmlBody := fmt.Sprintf(smtp.EmailVerificationBody(), name, emailNew, verifyURL)
	m.SetBody("text/html", htmlBody)

	d := gomail.NewDialer(smtpHost, smtpPort, smtpEmail, smtpPass)
	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send verification email: %v", err)
	}
}

// VerifyEmail verifies an email token and sets the new verified email.
func (s *Service) VerifyEmail(ctx context.Context, token string) (string, error) {
	username, pendingEmail, err := s.repo.VerifyEmailToken(ctx, token)
	if err != nil {
		return "", err
	}
	if pendingEmail == "" {
		return "", errors.New("no pending email to verify")
	}

	if err := s.repo.ApplyVerifiedEmail(ctx, username, pendingEmail); err != nil {
		return "", err
	}

	return username, nil
}

// CreateTopUpSession creates a pending topup record and generates a Xendit checkout invoice URL.
func (s *Service) CreateTopUpSession(ctx context.Context, username string, req TopUpRequest) (string, error) {
	if req.Amount.LessThan(decimal.NewFromFloat(MinTopUpAmount)) {
		return "", fmt.Errorf("amount must be at least %.2f", MinTopUpAmount)
	}
	if req.Amount.GreaterThan(decimal.NewFromFloat(MaxTopUpAmount)) {
		return "", fmt.Errorf("amount cannot exceed %.2f", MaxTopUpAmount)
	}

	feeAmount := decimal.NewFromFloat(ConvenienceFee)
	totalAmount := req.Amount.Add(feeAmount).InexactFloat64()

	cardNumber, email, err := s.repo.GetUserCardAndEmail(ctx, username)
	if err != nil {
		return "", ErrCardNotFound
	}

	topupID := fmt.Sprintf("TOPUP-%d", time.Now().UnixNano())

	if err := s.repo.CreatePendingTopUp(ctx, topupID, cardNumber, req.Amount, feeAmount); err != nil {
		return "", fmt.Errorf("create pending top-up: %w", err)
	}

	domain := "http://" + os.Getenv("SERVER_PORT") + ":" + os.Getenv("PORT")
	if domain == "http://" {
		domain = "http://127.0.0.1:3001"
	}

	var paymentMethods []string
	if req.PaymentMethod != "" {
		paymentMethods = []string{req.PaymentMethod}
	} else {
		paymentMethods = append([]string(nil), defaultPaymentMethods...)
	}

	xenditClient := xendit.NewClient(os.Getenv("XENDIT_SECRET_KEY"))
	data := *invoice.NewCreateInvoiceRequest(topupID, totalAmount)
	data.SetItems([]invoice.InvoiceItem{
		{
			Name:     "Unicard Top-Up",
			Price:    float32(req.Amount.InexactFloat64()),
			Quantity: 1,
		},
	})
	data.SetFees([]invoice.InvoiceFee{
		{
			Type:  "Convenience Fee",
			Value: float32(feeAmount.InexactFloat64()),
		},
	})
	data.SetPayerEmail(email)
	data.SetDescription(fmt.Sprintf("Unicard Top-Up (Card: %s)", cardNumber))
	data.SetPaymentMethods(paymentMethods)
	data.SetCurrency("PHP")
	data.SetInvoiceDuration(float32(15 * 60))
	data.SetSuccessRedirectUrl(domain + "/u/" + username + "/dashboard")
	data.SetFailureRedirectUrl(domain + "/u/" + username + "/topup")

	resp, _, xenditErr := xenditClient.InvoiceApi.CreateInvoice(ctx).
		CreateInvoiceRequest(data).
		Execute()

	if xenditErr != nil {
		return "", fmt.Errorf("xendit create invoice: %w", xenditErr)
	}

	return resp.GetInvoiceUrl(), nil
}

// ProcessXenditWebhook handles callback notifications from Xendit.
func (s *Service) ProcessXenditWebhook(ctx context.Context, payload XenditWebhookPayload) (string, error) {
	switch payload.Status {
	case "PAID", "SETTLED":
		return s.repo.ProcessPaidTopUp(ctx, payload.ExternalID, payload)
	case "PENDING":
		return s.repo.ProcessNonPaidTopUp(ctx, payload.ExternalID, payload.Status)
	default:
		return s.repo.ProcessNonPaidTopUp(ctx, payload.ExternalID, payload.Status)
	}
}
