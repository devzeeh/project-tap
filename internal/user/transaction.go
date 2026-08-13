package user

import (
	"fmt"
	"net/http"
	"strings"

	jsonwrite "project-tap/internal/pkg/handler"

	"github.com/shopspring/decimal"
)

// TransactionView renders the transaction.html template
func (h *Handler) TransactionView(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Transaction view is running...")

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

	h.Tpl.ExecuteTemplate(w, "transaction.html", data)
}
func (h *Handler) TransactionsJSONHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Username is required",
		})
		return
	}

	if !h.IsAuthorizedUser(r, username) {
		jsonwrite.WriteJSON(w, http.StatusForbidden, jsonwrite.APIResponse{
			Success: false,
			Message: "Forbidden: Insufficient permissions",
		})
		return
	}

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
		`
	rows, err := h.Store.Query(txnQuery, username, username)

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

	var transactions []TxnResponse
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t TxnResponse
			var description string
			var businessName string
			var merchantId string
			var pointsEarned decimal.Decimal
			var cardNumber string

			var createdAt string

			err := rows.Scan(&t.TransactionID, &t.TerminalID, &createdAt, &t.Type, &t.Amount, &t.Status, &description, &businessName, &merchantId, &pointsEarned, &cardNumber)
			if err != nil {
				fmt.Printf("Error scanning transaction row: %v\n", err)
				continue
			}

			t.Date = formatDate(createdAt)
			t.Time = formatTime(createdAt)

			t.Description = description

			if businessName != "" {
				t.MerchantName = businessName
			} else {
				t.MerchantName = "Transaction"
			}

			if merchantId != "" {
				t.MerchantID = merchantId
			} else {
				t.MerchantID = "N/A"
			}

			t.ServiceFee = 0.00
			t.PointsEarned = pointsEarned

			// Calculate Sender, Receiver
			isPayment := strings.ToLower(t.Type) == "payment"
			merchantStr := t.Description
			if t.TerminalID != "" && t.TerminalID != "N/A" {
				merchantStr += " (Terminal: " + t.TerminalID + ")"
			}
			cardStr := "My UniCard"
			if len(cardNumber) >= 4 {
				cardStr += " (**** " + cardNumber[len(cardNumber)-4:] + ")"
			}

			if isPayment {
				t.Sender = cardStr
				t.Receiver = merchantStr
			} else {
				t.Sender = merchantStr
				t.Receiver = cardStr
			}

			transactions = append(transactions, t)
		}
	}

	response := struct {
		Success      bool          `json:"success"`
		Transactions []TxnResponse `json:"transactions"`
	}{
		Success:      true,
		Transactions: transactions,
	}

	jsonwrite.WriteJSON(w, http.StatusOK, response)
}
