package merchant

import (
	"context"

	"github.com/shopspring/decimal"
	xendit "github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/payout"
)

// XenditPayoutWebhookPayload represents the expected payload from Xendit Payout webhook
type XenditPayoutWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		ReferenceID string          `json:"reference_id"`
		Status      string          `json:"status"` // SUCCEEDED, FAILED
		ChannelCode string          `json:"channel_code"`
		Amount      decimal.Decimal `json:"amount"`
		FailureCode string          `json:"failure_code,omitempty"`
	} `json:"data"`
}

// XenditPayoutGateway implements the PayoutGateway interface using the official Xendit SDK.
type XenditPayoutGateway struct {
	client payout.PayoutApi
}

// NewXenditPayoutGateway initializes a new Xendit API client with the provided API key.
func NewXenditPayoutGateway(apiKey string) *XenditPayoutGateway {
	xc := xendit.NewClient(apiKey)
	return &XenditPayoutGateway{client: payout.NewPayoutApi(xc)}
}

// CreatePayout submits a payout request to Xendit using the provided transaction details.
func (g *XenditPayoutGateway) CreatePayout(ctx context.Context, txnID, channelCode, accountNumber, accountHolderName string, amount float64, description string) error {
	channelProps := payout.NewDigitalPayoutChannelProperties(accountNumber)
	channelProps.SetAccountHolderName(accountHolderName)

	req := payout.NewCreatePayoutRequest(txnID, channelCode, *channelProps, float32(amount), "PHP")
	req.SetDescription(description)

	_, _, err := g.client.CreatePayout(ctx).
		IdempotencyKey(txnID).
		CreatePayoutRequest(*req).
		Execute()
	return err
}
