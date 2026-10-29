package user

import (
	"context"

	xendit "github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/invoice"
)

type XenditInvoiceGateway struct {
	client invoice.InvoiceApi
}

func NewXenditInvoiceGateway(apiKey string) *XenditInvoiceGateway {
	xc := xendit.NewClient(apiKey)
	return &XenditInvoiceGateway{client: xc.InvoiceApi}
}

func (g *XenditInvoiceGateway) CreateInvoice(ctx context.Context, p CreateInvoiceParams) (string, error) {
	data := *invoice.NewCreateInvoiceRequest(p.ExternalID, p.TotalAmount)
	data.SetItems([]invoice.InvoiceItem{
		{Name: "Unicard Top-Up", Price: float32(p.ItemAmount), Quantity: 1},
	})
	data.SetFees([]invoice.InvoiceFee{
		{Type: "Convenience Fee", Value: float32(p.FeeAmount)},
	})
	data.SetPayerEmail(p.PayerEmail)
	data.SetDescription(p.Description)
	data.SetPaymentMethods(p.PaymentMethods)
	data.SetCurrency("PHP")
	data.SetInvoiceDuration(float32(invoiceDuration))
	data.SetSuccessRedirectUrl(p.SuccessURL)
	data.SetFailureRedirectUrl(p.FailureURL)

	resp, _, err := g.client.CreateInvoice(ctx).
		CreateInvoiceRequest(data).
		Execute()
	if err != nil {
		return "", err
	}

	return resp.GetInvoiceUrl(), nil
}