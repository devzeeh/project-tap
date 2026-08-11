package user

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	jsonwrite "project-tap/internal/pkg/handler"
)

func (h *Handler) CardView(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Card view is running...")

	username := r.PathValue("username")
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

	h.Tpl.ExecuteTemplate(w, "card.html", data)
}

// UpdateCardStatusRequest represents the payload for updating card status.
type UpdateCardStatusRequest struct {
	Status string `json:"status"`
}
func (h *Handler) UpdateCardStatus(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	var req UpdateCardStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var userID string
	err := h.Store.QueryRow("SELECT user_id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Make sure status is valid
	if req.Status != "active" && req.Status != "inactive" && req.Status != "blocked" && req.Status != "lost" {
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}

	_, err = h.Store.Exec("UPDATE cards SET status = ? WHERE user_id = ?", req.Status, userID)
	if err != nil {
		http.Error(w, "Failed to update card status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}
func (h *Handler) RequestReplacement(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	var userID string
	err := h.Store.QueryRow("SELECT user_id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}

	var balance float64
	var cardNumber string
	err = h.Store.QueryRow("SELECT balance, card_number FROM cards WHERE user_id = ?", userID).Scan(&balance, &cardNumber)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusNotFound, jsonwrite.APIResponse{
			Success: false,
			Message: "Card not found",
		})
		return
	}

	if balance < 150.0 {
		jsonwrite.WriteJSON(w, http.StatusBadRequest, jsonwrite.APIResponse{
			Success: false,
			Message: "Insufficient balance. Replacement fee is 150 PHP.",
		})
		return
	}

	// Deduct balance and set card to locked (represented as 'blocked')
	// Using ExecTx if available, or just manual Begin
	tx, err := h.Store.Begin()
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Database error",
		})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE cards SET balance = balance - 150.0, status = 'blocked' WHERE user_id = ?", userID)
	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to update card",
		})
		return
	}

	// Insert transaction record for the fee
	txnID := fmt.Sprintf("REP-%s-%d", cardNumber, time.Now().Unix())
	_, err = tx.Exec(`
		INSERT INTO transactions (transaction_id, card_number, user_id, transaction_type, amount, status, description)
		VALUES (?, ?, ?, 'payment', 150.0, 'completed', 'Card Replacement Fee')
	`, txnID, cardNumber, userID)

	if err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to record transaction",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		jsonwrite.WriteJSON(w, http.StatusInternalServerError, jsonwrite.APIResponse{
			Success: false,
			Message: "Failed to commit transaction",
		})
		return
	}

	jsonwrite.WriteJSON(w, http.StatusOK, jsonwrite.APIResponse{
		Success: true,
		Message: "Card replacement requested successfully. Card is now locked.",
	})
}
