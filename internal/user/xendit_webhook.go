package user

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// XenditWebhookPayload represents the expected payload from Xendit Invoice webhook
type XenditWebhookPayload struct {
	ID                     string          `json:"id"`
	ExternalID             string          `json:"external_id"`
	UserID                 string          `json:"user_id"`
	IsHigh                 bool            `json:"is_high"`
	PaymentMethod          string          `json:"payment_method"`
	Status                 string          `json:"status"`
	MerchantName           string          `json:"merchant_name"`
	Amount                 decimal.Decimal `json:"amount"`
	PaidAmount             decimal.Decimal `json:"paid_amount"`
	BankCode               string          `json:"bank_code"`
	PaidAt                 string          `json:"paid_at"`
	PayerEmail             string          `json:"payer_email"`
	Description            string          `json:"description"`
	AdjustedReceivedAmount decimal.Decimal `json:"adjusted_received_amount"`
	FeesPaidAmount         decimal.Decimal `json:"fees_paid_amount"`
	Updated                string          `json:"updated"`
	Created                string          `json:"created"`
	Currency               string          `json:"currency"`
	PaymentChannel         string          `json:"payment_channel"`
	PaymentDestination     string          `json:"payment_destination"`
}

// XenditWebhook handles incoming webhook notifications from Xendit for invoice payments.
// It validates the callback token, processes the payment payload, and updates the user's balance.
// more info here: https://developers.xendit.co/docs/invoices#handling-invoice-completion-via-webhooks
func (h *Handler) XenditWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify Xendit Callback Token
	xenditToken := os.Getenv("XENDIT_WEBHOOK_KEY")
	callbackToken := r.Header.Get("x-callback-token")

	// If no token is configured, skip validation for development, but it's recommended to have it
	if xenditToken != "" && callbackToken != xenditToken {
		log.Println("Invalid x-callback-token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// read body from xendit invoice webhook
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Failed to read body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// parse body
	var payload XenditWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Println("Failed to parse webhook JSON:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// switch case for payment status
	switch payload.Status {
	case "PAID", "SETTLED":
		externalID := payload.ExternalID // This maps to our topup_id

		// Start Database Transaction
		err = h.Store.ExecTx(r.Context(), func(tx *sql.Tx) error {
			var topUp XenditWebhookPayload
			var cardNumber string
			var convenienceFee float64
			var currentStatus string

			// Fetch the top-up record
			err := tx.QueryRow(`SELECT card_number, amount, convenience_fee, status FROM top_ups WHERE topup_id = ?`, externalID).Scan(&cardNumber, &topUp.Amount, &convenienceFee, &currentStatus)
			if err != nil {
				log.Println("Failed to find top-up record or invalid external_id:", err)
				return nil // Ignore if not found, don't rollback, just return success to not re-trigger webhook
			}

			// Prevent double processing
			if currentStatus == "completed" {
				log.Println("Top-up already completed, skipping.")
				return nil // Ignore if already processed
			}

			// Update the User's Balance
			if _, err := tx.Exec(`UPDATE cards SET balance = balance + ? WHERE card_number = ?`, topUp.Amount, cardNumber); err != nil {
				log.Println("Failed to update card balance:", err)
				return fmt.Errorf("failed to update card balance")
			}

			// Mark the top-up ledger as completed
			if _, err := tx.Exec(`UPDATE top_ups SET status = 'completed' WHERE topup_id = ?`, externalID); err != nil {
				log.Println("Failed to update top_ups status:", err)
				return fmt.Errorf("failed to update top_ups status")
			}

			// Upsert the transaction: update if a pending one exists, otherwise insert
			res, err := tx.Exec(`UPDATE transactions SET status = 'completed', description = 'Successful topup via Xendit' WHERE card_number = ? AND transaction_type = 'topup' AND status = 'pending' AND amount = ? ORDER BY created_at DESC LIMIT 1`, cardNumber, topUp.Amount)
			if err != nil {
				log.Println("Failed to update transactions status:", err)
				return fmt.Errorf("failed to update transactions status")
			}

			rowsAffected, _ := res.RowsAffected()
			if rowsAffected == 0 {
				transactionID := fmt.Sprintf("TX-%d", time.Now().UnixNano())
				var userID string
				_ = tx.QueryRow("SELECT user_id FROM cards WHERE card_number = ?", cardNumber).Scan(&userID)
				queryTx := `INSERT INTO transactions (transaction_id, card_number, user_id, merchant_id, terminal_id, transaction_type, amount, service_fee, processed_by, description, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
				if _, err := tx.Exec(queryTx, transactionID, cardNumber, userID, "xendit", "xendit", "topup", topUp.Amount, convenienceFee, "xendit", "Successful topup via Xendit", "completed"); err != nil {
					log.Println("Failed to insert transaction:", err)
					return fmt.Errorf("failed to insert transaction")
				}
			}

			log.Printf("Successfully loaded ₱%s onto card %s via Xendit", topUp.Amount, cardNumber)
			return nil
		})

		if err != nil {
			log.Println("Transaction failed:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	// log if payment failed, expired, or canceled
	case "PENDING":
		_, _ = h.Store.Exec(`UPDATE top_ups SET status = ? WHERE topup_id = ?`, strings.ToLower(payload.Status), payload.ExternalID)

	case "EXPIRED", "FAILED", "CANCELLED":
		log.Printf("Payment Status for external ID: %s, status: %s", payload.ExternalID, payload.Status)
		// Update the database records to failed/expired so users see it in top_up history
		_, _ = h.Store.Exec(`UPDATE top_ups SET status = ? WHERE topup_id = ?`, strings.ToLower(payload.Status), payload.ExternalID)

		// Remove the pending transaction from the transactions ledger so it doesn't clutter the history
		var cardNumber string
		var amount float64
		if err := h.Store.QueryRow(`SELECT card_number, amount FROM top_ups WHERE topup_id = ?`, payload.ExternalID).Scan(&cardNumber, &amount); err == nil {
			_, errDelete := h.Store.Exec(`DELETE FROM transactions WHERE card_number = ? AND transaction_type = 'topup' AND status = 'pending' AND amount = ? ORDER BY created_at DESC LIMIT 1`, cardNumber, amount)
			if errDelete != nil {
				log.Println("Error deleting pending transaction:", errDelete)
			}
		}
	}

	// Always 200 OK to Xendit
	w.WriteHeader(http.StatusOK)
}
