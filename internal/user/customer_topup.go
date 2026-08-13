package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	jsonwrite "project-tap/internal/pkg/handler"

	"github.com/shopspring/decimal"
	xendit "github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/invoice"
)

// struct for topup request only, for api call not for saving in db
type TopUpRequest struct {
	CardNumber    string          `json:"card_number"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentMethod string          `json:"payment_method"`
}

// struct for topup record, for saving in db and for webhook callback processing
// note the fields that match the db schema
// the external_id is encoded in the external_id field of the xendit invoice
// this is necessary because xendit v1 doesn't have a metadata field
type TopUpRecord struct {
	TopupID        string          `json:"topup_id" db:"topup_id"`
	CardNumber     string          `json:"card_number" db:"card_number"`
	Amount         decimal.Decimal `json:"amount" db:"amount"`
	ConvenienceFee decimal.Decimal `json:"convenience_fee" db:"convenience_fee"`
	GatewayCost    decimal.Decimal `json:"gateway_cost" db:"gateway_cost"`
	PaymentMethod  string          `json:"payment_method" db:"payment_method"`
}

// declare conveniece fee as constant
const ConvenienceFee = 15.00

// TopUpView displays the top-up page for a user
func (h *Handler) TopUpView(w http.ResponseWriter, r *http.Request) {
	fmt.Println("TopUp view is running...")

	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.GetDashboardUser(username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	}

	h.Tpl.ExecuteTemplate(w, "customer_topup.html", data)
}
func (h *Handler) CreateXenditInvoice(w http.ResponseWriter, r *http.Request) {
	// get username from url parameter
	username := r.PathValue("username")
	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

	// get request body
	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	// check if amount is at least 50 pesos
	if req.Amount.LessThan(decimal.NewFromFloat(50.0)) {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Amount must be at least 50.00",
		})
		return
	}

	if req.Amount.GreaterThan(decimal.NewFromFloat(2000.0)) {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Amount cannot exceed 2,000.00",
		})
		return
	}

	// set fee amount and total amount
	// total of the topup
	topupAmount := req.Amount
	//convenience fee for now is fixed at 15 pesos
	feeAmount := decimal.NewFromFloat(ConvenienceFee)
	// total amount to be charged to the user's payment method
	totalAmount := topupAmount.Add(feeAmount).InexactFloat64()

	// Fetch card number and email securely from DB instead of trusting the frontend
	var cardNumber, email string
	err := h.Store.QueryRow(`
		SELECT c.card_number, u.email 
		FROM cards c 
		JOIN users u ON c.user_id = u.user_id 
		WHERE u.username = ? LIMIT 1
	`, username).Scan(&cardNumber, &email)

	if err != nil {
		// print error and return
		log.Println("Failed to find card for user:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to find card for user",
		})
		return
	}

	// set domain
	domain := "http://" + os.Getenv("SERVER_PORT") + ":" + os.Getenv("PORT")
	// Fallback if domain is malformed
	if domain == "http://" {
		domain = "http://127.0.0.1:3000"
	}

	// Generate Unique IDs
	topupID := fmt.Sprintf("TOPUP-%d", time.Now().UnixNano())

	// Start Database Transaction to insert PENDING records
	tx, dbErr := h.Store.Begin()
	if dbErr != nil {
		log.Println("Failed to start transaction:", dbErr)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Internal Server Error",
		})
		return
	}

	// Insert into Loading Ledger (top_ups table) with status = 'pending'
	queryTopUp := `INSERT INTO top_ups (topup_id, card_number, amount, convenience_fee, gateway_cost, payment_method, handled_by, external_id, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := tx.Exec(queryTopUp, topupID, cardNumber, topupAmount, feeAmount, 0.0, "xendit", "payment gateway", topupID, "pending"); err != nil {
		tx.Rollback()
		log.Println("Failed to record pending top-up:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to record pending top-up",
		})
		return
	}

	// Commit the pending records
	if err := tx.Commit(); err != nil {
		log.Println("Failed to finalize pending records:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to finalize database changes",
		})
		return
	}

	var paymentMethods []string
	if req.PaymentMethod != "" {
		paymentMethods = []string{req.PaymentMethod}
	} else {
		paymentMethods = []string{"CREDIT_CARD", "UBP_DIRECT_DEBIT", "BPI_DIRECT_DEBIT", "QRPH", "GCASH",
			"PAYMAYA", "GRABPAY", "SHOPEEPAY", "7ELEVEN"}
	}

	// set xendit secret key
	xenditClient := xendit.NewClient(os.Getenv("XENDIT_SECRET_KEY"))

	// The Xendit ExternalID will cleanly map to our topup_id
	externalID := topupID

	// create xendit invoice struct with parameters
	data := *invoice.NewCreateInvoiceRequest(externalID, totalAmount)
	data.SetItems([]invoice.InvoiceItem{
		{
			Name:     "Unicard Top-Up",
			Price:    float32(topupAmount.InexactFloat64()),
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
	data.SetInvoiceDuration(float32(15 * 60)) // 15 minutes invoice expiration
	data.SetSuccessRedirectUrl(domain + "/u/" + username + "/dashboard")
	data.SetFailureRedirectUrl(domain + "/u/" + username + "/topup")

	// creating the checkout session
	resp, _, xenditErr := xenditClient.InvoiceApi.CreateInvoice(context.Background()).
		CreateInvoiceRequest(data).
		Execute()

	if xenditErr != nil {
		log.Println("Failed to create checkout session:", xenditErr)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to create checkout session",
		})
		return
	}

	// Handle success case
	// return the checkout session url to the frontend
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Checkout session created successfully",
		Data:    map[string]string{"url": resp.GetInvoiceUrl()},
	})

	// log the response from xendit. Can be useful for debugging only
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `InvoiceApi.CreateInvoice`: %s\n", string(out))

	// log the checkout session url
	log.Println("Checkout session created successfully:", resp.GetInvoiceUrl())
}
func (h *Handler) SaveTopUpToDatabase(w http.ResponseWriter, r *http.Request) {
	log.Println("SaveTopUpToDatabase running...")

	var req TopUpRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}

	// Generate Unique IDs for both ledgers
	topupID := fmt.Sprintf("TOPUP-%d", time.Now().UnixNano())
	transactionID := fmt.Sprintf("TX-%d", time.Now().UnixNano())

	// Start the Database Transaction
	tx, err := h.Store.Begin()
	if err != nil {
		log.Println("Failed to start transaction:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to start transaction",
		})
		return
	}

	// SAFETY NET: This automatically rolls back if the function exits before tx.Commit()
	defer tx.Rollback()

	// Update User Balance (cards table)
	if _, err := tx.Exec(`UPDATE cards SET balance = balance + ? WHERE card_number = ?`, req.Amount, req.CardNumber); err != nil {
		log.Println("Failed to update user balance:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to update user balance",
		})
		return // The defer statement will automatically handle the rollback
	}

	// Insert into Loading Ledger (top_ups table)
	queryTopUp := `INSERT INTO top_ups (topup_id, card_number, amount, convenience_fee, gateway_cost, payment_method) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := tx.Exec(queryTopUp, topupID, req.CardNumber, req.Amount, req.ConvenienceFee, req.GatewayCost, req.PaymentMethod); err != nil {
		log.Println("Failed to record top-up:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to record top-up",
		})
		return
	}

	// Insert into Spending Ledger (transactions table)
	// *Note: Adjust 'category' if your enum doesn't include 'top_up'
	// Fetch user_id for the transaction log
	var userID string
	_ = tx.QueryRow("SELECT user_id FROM cards WHERE card_number = ?", req.CardNumber).Scan(&userID)

	queryTx := `INSERT INTO transactions (transaction_id, card_number, user_id, merchant_id, terminal_id, transaction_type, amount, service_fee, processed_by, status, description) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// "xendit" as the merchant_id since this is an internal load, not a retail/Fare purchase
	if _, err := tx.Exec(queryTx, transactionID, req.CardNumber, userID, sql.NullString{}, sql.NullString{}, "topup", req.Amount, req.ConvenienceFee, sql.NullString{}, "pending", "Topup initiated"); err != nil {
		log.Println("Failed to record global transaction:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to record global transaction",
		})
		return
	}

	// THE MOST IMPORTANT PART: Commit the transaction!
	if err := tx.Commit(); err != nil {
		log.Println("Failed to finalize database changes:", err)
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to finalize database changes",
		})
		return
	}

	// Success Response
	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Top-up fully processed and saved",
	})
	log.Println("Transaction saved successfully")
}
