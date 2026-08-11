package xenditclient

import (
	"context"
	"fmt"
	"os"

	"github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/balance_and_transaction"
)

// GetAllTransactions fetches all transactions from Xendit API.
func GetAllTransactions() (*balance_and_transaction.TransactionsResponse, error) {
	// Fallback to the known secret key if ENV is not set for the test environment
	secretKey := os.Getenv("XENDIT_SECRET_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("XENDIT_SECRET_KEY is not set")
	}

	xenditClient := xendit.NewClient(secretKey)
	transactionClient := balance_and_transaction.NewTransactionApi(xenditClient)

	req := transactionClient.GetAllTransactions(context.Background())
	
	resp, _, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions from Xendit: %w", err)
	}

	return resp, nil
}
