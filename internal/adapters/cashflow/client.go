package cashflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ctxKey string

const AuthHeaderKey ctxKey = "authorization"

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) ScheduleSale(ctx context.Context, orderID, customerID, methodID, termID string, amount float64, at time.Time) error {
	return c.schedule(ctx, map[string]any{
		"direction":         "IN",
		"amount":            amount,
		"payment_method_id": methodID,
		"payment_term_id":   termID,
		"reference_type":    "SALE",
		"reference_id":      orderID,
		"party_id":          customerID,
		"occurred_at":       at.UTC().Format(time.RFC3339),
	})
}

// CancelSchedule clears whatever was scheduled for this order — used when the order itself
// is cancelled or deleted, so a removed order doesn't leave a "Venda" entry behind on the
// Fluxo de caixa screen forever (see cashflow-service's own ReplaceSchedule, which this
// mirrors: deleting with nothing to reinsert is exactly a cancel).
func (c *Client) CancelSchedule(ctx context.Context, orderID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/schedule/SALE/"+orderID, nil)
	if err != nil {
		return err
	}
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cashflow: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) schedule(ctx context.Context, body map[string]any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/schedule", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cashflow: %s", strings.TrimSpace(string(b)))
	}
	return nil
}
