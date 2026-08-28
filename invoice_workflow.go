package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type Order struct {
	ID          string        `json:"id"`
	Customer    Customer      `json:"customer"`
	Checkout    Checkout      `json:"checkout"`
	Fulfillment Fulfillment   `json:"fulfillment"`
	Items       []LineItem    `json:"items"`
	Receipts    []Receipt     `json:"receipts,omitempty"`
	Updates     []OrderUpdate `json:"updates,omitempty"`
}

type Customer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Checkout struct {
	Status   string `json:"status"`
	Currency string `json:"currency"`
	Total    string `json:"total"`
}

type Fulfillment struct {
	Status     string `json:"status"`
	TrackingID string `json:"tracking_id"`
}

type LineItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Amount   string `json:"amount"`
}

type Receipt struct {
	Kind string `json:"kind"`
	PDF  string `json:"pdf"`
}

type OrderUpdate struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type PDFClient interface {
	Generate(context.Context, string, string) (string, error)
	Watermark(context.Context, string, string, string) (string, error)
}

type Service struct {
	pdf PDFClient
}

func NewService(pdf PDFClient) *Service {
	return &Service{pdf: pdf}
}

func (s *Service) CreateInvoice(ctx context.Context, order Order) (Order, error) {
	if order.ID == "" || order.Customer.Name == "" || len(order.Items) == 0 {
		return order, errors.New("order id, customer, and items are required")
	}
	if order.Checkout.Status != "paid" {
		return order, errors.New("invoice requires a paid checkout")
	}
	if order.Fulfillment.Status != "fulfilled" {
		return order, errors.New("invoice requires fulfilled items")
	}

	html, err := renderInvoice(order)
	if err != nil {
		return order, err
	}
	generatedPDF, err := s.pdf.Generate(ctx, html, order.ID)
	if err != nil {
		return order, err
	}
	finalPDF, err := s.pdf.Watermark(ctx, generatedPDF, "PAID", order.ID)
	if err != nil {
		return order, err
	}

	order.Receipts = append(order.Receipts, Receipt{Kind: "invoice", PDF: finalPDF})
	order.Updates = append(order.Updates, OrderUpdate{
		Type:    "invoice_ready",
		Message: "Invoice is ready for " + order.Customer.Email,
	})
	return order, nil
}

func (s *Service) HandleInvoice(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order JSON"})
		return
	}

	updated, err := s.CreateInvoice(r.Context(), order)
	if err != nil {
		status := http.StatusUnprocessableEntity
		var apiErr *InfraiError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			status = apiErr.Status
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, updated)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

var invoiceTemplate = template.Must(template.New("invoice").Parse(`<!doctype html>
<html><body><h1>Invoice {{.ID}}</h1><p>Customer: {{.Customer.Name}}</p>
<table><tr><th>Item</th><th>Qty</th><th>Amount</th></tr>
{{range .Items}}<tr><td>{{.Name}}</td><td>{{.Quantity}}</td><td>{{.Amount}}</td></tr>{{end}}
</table><p>Total: {{.Checkout.Total}} {{.Checkout.Currency}}</p>
<p>Tracking: {{.Fulfillment.TrackingID}}</p></body></html>`))

func renderInvoice(order Order) (string, error) {
	var output strings.Builder
	if err := invoiceTemplate.Execute(&output, order); err != nil {
		return "", fmt.Errorf("render invoice: %w", err)
	}
	return output.String(), nil
}
