package invoice

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingPDF struct {
	calls []string
}

func (f *recordingPDF) Generate(_ context.Context, _ string, _ string) (string, error) {
	f.calls = append(f.calls, "generate")
	return "https://files.example/generated.pdf", nil
}

func (f *recordingPDF) Watermark(_ context.Context, pdf, _ string, _ string) (string, error) {
	f.calls = append(f.calls, "watermark:"+pdf)
	return "https://files.example/invoice.pdf", nil
}

func TestCreateInvoiceBusinessDecision(t *testing.T) {
	tests := []struct {
		name      string
		checkout  string
		fulfill   string
		wantErr   string
		wantCalls []string
		wantReady bool
	}{
		{name: "paid fulfilled order gets invoice", checkout: "paid", fulfill: "fulfilled", wantCalls: []string{"generate", "watermark:https://files.example/generated.pdf"}, wantReady: true},
		{name: "unpaid order is held", checkout: "pending", fulfill: "fulfilled", wantErr: "invoice requires a paid checkout"},
		{name: "unfulfilled order is held", checkout: "paid", fulfill: "packing", wantErr: "invoice requires fulfilled items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := &recordingPDF{}
			service := NewService(pdf)
			order := validOrder()
			order.Checkout.Status = tt.checkout
			order.Fulfillment.Status = tt.fulfill

			got, err := service.CreateInvoice(context.Background(), order)
			if tt.wantErr != "" {
				if !errors.Is(err, errors.New(tt.wantErr)) && (err == nil || err.Error() != tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("CreateInvoice() error = %v", err)
			}
			if !reflect.DeepEqual(pdf.calls, tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", pdf.calls, tt.wantCalls)
			}
			ready := len(got.Receipts) == 1 && len(got.Updates) == 1 && got.Updates[0].Type == "invoice_ready"
			if ready != tt.wantReady {
				t.Fatalf("invoice ready = %v, want %v", ready, tt.wantReady)
			}
		})
	}
}

func validOrder() Order {
	return Order{
		ID:       "ord_1042",
		Customer: Customer{Name: "Ada Lovelace", Email: "ada@example.com"},
		Checkout: Checkout{Status: "paid", Currency: "USD", Total: "79.00"},
		Fulfillment: Fulfillment{
			Status:     "fulfilled",
			TrackingID: "track_88",
		},
		Items: []LineItem{{Name: "Mechanical keyboard", Quantity: 1, Amount: "79.00"}},
	}
}
