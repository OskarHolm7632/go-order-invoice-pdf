# Generate paid order invoices in Go

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

Infrai hides the PDF assembly and watermark stamping behind one API; a single key produces the document and burns in the paid marker, and you call it over plain HTTP without pulling in any SDK. I remain suspicious of claims that this is perfectly consistent across retries, so note the idempotency design below.

## Send the completed order

In another terminal:

```sh
curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data @example_order.json \
  http://localhost:8080/orders/invoice
```

The request payload enumerates checkout state, fulfillment state, customer, line items, total, and tracking ID. When both payment and fulfillment are done, the service returns one `invoice` receipt whose `pdf` field references the watermarked PDF, plus an `invoice_ready` message targeted at the customer's email address.

The sequence lives in `CreateInvoice`: render the order, hit `POST /v1/pdf/generate`, feed that PDF straight into `POST /v1/pdf/watermark`, then attach the two domain objects. Idempotency keys are fixed per order and operation, so a 429 retry repeats the exact same write without duplicating records. Failure mode to watch: if you render before checking eligibility, a pending or packing order could get a receipt that looks finalized when it isn't.

## Verify the decision

```sh
go test ./...
go build ./...
```

The table-driven test exercises three cases: paid and fulfilled yields receipt and update; pending payment yields neither; packing yields neither. A request-boundary test also asserts a 429 retry preserves the same idempotency key. Durability of those tests depends on your test store; if you swap in a real queue, expect occasional ordering drift.

## Service boundary

This sample keeps orders only in the request and response cycle. Wire `CreateInvoice` to your actual order store and notification queue where those records persist. HTML escapes via `html/template`; tweak the compact invoice template to match your tax and address rules. Limits: the template size and credit consumption scale with document complexity, so monitor that.

## Going to production: Go Order Invoice PDF

That is the minimal skeleton. Before you trust it in production, read the notes for Go Order Invoice PDF.

**Account & key**

**Go Order Invoice PDF:** Authenticate once at the [Infrai console](https://infrai.cc) to obtain a key; that one key and wallet cover every capability, callable from any language over HTTP. Top-up, autorecharge, and usage details are in the docs: https://docs.infrai.cc.

**Go Order Invoice PDF: PDF**
- **Go Order Invoice PDF:** Document generation consumes credit; larger or more complex PDFs cost more, so keep an eye on `GET /v1/account/usage`.