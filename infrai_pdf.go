package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type InfraiPDFClient struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InfraiError struct {
	Status  int
	Code    string
	Message string
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

func NewInfraiPDFClient(baseURL, key string, client *http.Client) *InfraiPDFClient {
	return &InfraiPDFClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    client,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *InfraiPDFClient) Generate(ctx context.Context, html, orderID string) (string, error) {
	body := map[string]any{
		"html":        html,
		"page_size":   "A4",
		"orientation": "portrait",
		"store":       false,
	}
	return c.post(ctx, "/v1/pdf/generate", body, "invoice-generate-"+orderID)
}

func (c *InfraiPDFClient) Watermark(ctx context.Context, pdf, text, orderID string) (string, error) {
	body := map[string]any{
		"pdf":      pdf,
		"text":     text,
		"opacity":  0.16,
		"position": "center",
	}
	return c.post(ctx, "/v1/pdf/watermark", body, "invoice-watermark-"+orderID)
}

func (c *InfraiPDFClient) post(ctx context.Context, path string, body any, idempotencyKey string) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.http.Do(req)
		if err != nil {
			return "", fmt.Errorf("send request: %w", err)
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(responseBody, &env)
		if decodeErr == nil && res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return "", err
			}
			continue
		}
		if decodeErr != nil {
			return "", fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if !env.OK {
			apiErr := env.Error
			if apiErr == nil {
				apiErr = &apiError{Code: "REQUEST_REJECTED", Message: "request was rejected"}
			}
			return "", &InfraiError{Status: res.StatusCode, Code: apiErr.Code, Message: apiErr.Message}
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return "", fmt.Errorf("infrai transport status %d", res.StatusCode)
		}

		var result struct {
			URL string `json:"url"`
			PDF string `json:"pdf"`
		}
		if err := json.Unmarshal(env.Data, &result); err != nil {
			return "", fmt.Errorf("decode response data: %w", err)
		}
		if result.PDF != "" {
			return result.PDF, nil
		}
		if result.URL != "" {
			return result.URL, nil
		}
		return "", fmt.Errorf("response data has no PDF location")
	}
	return "", fmt.Errorf("retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}
