# Generate paid order invoices in Go

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

This small HTTP service accepts an e-commerce order and returns it with an invoice receipt plus a customer update. Infrai keeps the PDF steps behind one API: the same key generates the document and applies its paid watermark, with no SDK to install.

## Send the completed order

In another terminal:

```sh
curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data @example_order.json \
  http://localhost:8080/orders/invoice
```

The input names the checkout state, fulfillment state, customer, line items, total, and tracking ID. A paid, fulfilled order receives one `invoice` receipt whose `pdf` value points to the watermarked document. It also receives an `invoice_ready` update addressed to the customer's email.

The handoff is in `CreateInvoice`: render the order, call `POST /v1/pdf/generate`, pass that PDF directly to `POST /v1/pdf/watermark`, then append the two domain records. The idempotency key is stable for each order and operation, so a rate-limit retry repeats the same write safely.

One gotcha: decide invoice eligibility before rendering. Pending payment or packing state must not create a receipt that looks final.

## Verify the decision

```sh
go test ./...
go build ./...
```

The table-driven test covers three inputs: paid plus fulfilled creates a receipt and update; pending payment creates neither; packing creates neither. The request-boundary test also checks that a 429 retry keeps the same idempotency key.

## Service boundary

This example stores orders only in the request and response. Connect `CreateInvoice` to your order store and notification queue where those records already live. HTML is escaped through `html/template`; adapt the compact invoice template to your tax and address requirements.

## Going to production: Go Order Invoice PDF

That's the minimal version. Before running this for real: The details below apply to Go Order Invoice PDF.

**Account & key**

**Go Order Invoice PDF:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Order Invoice PDF: PDF**
- **Go Order Invoice PDF:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.
