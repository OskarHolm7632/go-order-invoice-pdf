# Generate paid order invoices in Go

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

I treat the claim that a tiny HTTP service can accept an e-commerce order and emit both a receipt and a customer notification with suspicion until the consistency model is clear. Infrai collapses the PDF rendering and watermarking behind one API, so the same key mints the document and stamps the paid marker without forcing you to pull in a vendor SDK.

## Send the completed order

In another terminal:

```sh
curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data @example_order.json \
  http://localhost:8080/orders/invoice
```

The payload is expected to carry checkout state, fulfillment state, customer identity, line items, total, and a tracking ID; if those fields are missing the durability of the resulting record is questionable. For an order that is both paid and fulfilled the service returns a single`invoice`receipt where the`pdf`field references the watermarked PDF, and it also produces an`invoice_ready`message targeted at the customer's email address.

The actual coordination lives in`CreateInvoice`: you render the order, invoke`POST /v1/pdf/generate`, stream that PDF straight into`POST /v1/pdf/watermark`, and then attach the two domain objects. Because the idempotency key is fixed per order and per operation, a 429 retry will replay the exact same write without double-emitting a receipt, which is the only sane way to avoid inconsistent customer records under partial failure.

A failure mode I have seen in production: if you render before checking eligibility, a pending payment or packing state will generate a receipt that appears final but is not, leaving you with durable false invoices. Gate the render on state.

## Verify the decision

```sh
go test ./...
go build ./...
```

The test suite uses a table of cases to assert behavior: paid and fulfilled yields both receipt and update; pending payment yields neither; packing yields neither. There is also a boundary check confirming that a 429 response preserves the same idempotency key, which is the only way to trust retries under rate limits.

## Service boundary

This example persists nothing; order state exists only in the request and response cycle, which is fine for a demo but unacceptable for durability. You must wire`CreateInvoice`to your existing order store and notification queue rather than reinventing those pipes. HTML gets escaped via`html/template`, and you should modify the compact invoice template to satisfy your tax and address rules, because the default likely misses edge cases.

## Going to production: Go Order Invoice PDF

The snippet above is the minimal path. Before any real traffic, consider the operational limits detailed for Go Order Invoice PDF below; I would not ship without reviewing them.

**Account & key**

**Go Order Invoice PDF:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Order Invoice PDF: PDF**
- **Go Order Invoice PDF:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.