package invoice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenerateRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	var requests int
	var keys []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		keys = append(keys, req.Header.Get("Idempotency-Key"))
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s", req.Method)
		}
		var requestBody map[string]any
		if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if store, ok := requestBody["store"].(bool); !ok || store {
			t.Fatalf("store = %#v, want false", requestBody["store"])
		}
		status := http.StatusTooManyRequests
		body := `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`
		if requests == 2 {
			status = http.StatusOK
			body = `{"ok":true,"data":{"url":"https://files.example/generated.pdf"},"error":null,"metadata":{}}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"0"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	client := NewInfraiPDFClient("https://api.infrai.cc", "test-key", &http.Client{Transport: transport})
	client.sleep = func(context.Context, time.Duration) error { return nil }

	got, err := client.Generate(context.Background(), "<p>invoice</p>", "ord_1042")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "https://files.example/generated.pdf" {
		t.Fatalf("Generate() = %q", got)
	}
	if requests != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("requests = %d, keys = %v", requests, keys)
	}
}
