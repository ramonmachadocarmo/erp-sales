package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"erp/services/sales-service/internal/domain"
)

type ctxKey string

const AuthHeaderKey ctxKey = "authorization"

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *Client) EnsureCustomer(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/customers/"+id, nil)
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
	if resp.StatusCode == http.StatusNotFound {
		return domain.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) DefaultWarehouse(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/settings/default_warehouse_id", nil)
	if err != nil {
		return "", err
	}
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	var st struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return "", err
	}
	return strings.TrimSpace(st.Value), nil
}

func (c *Client) GetCenter(ctx context.Context, id string) (domain.Center, error) {
	var out domain.Center
	err := c.getJSON(ctx, "/centers/"+id, &out)
	return out, err
}

func (c *Client) GetVehicle(ctx context.Context, id string) (domain.Vehicle, error) {
	var out domain.Vehicle
	err := c.getJSON(ctx, "/vehicles/"+id, &out)
	return out, err
}

func (c *Client) SearchAddress(ctx context.Context, a domain.OrderAddress) (float64, float64, error) {
	q := url.Values{}
	q.Set("street", a.Street)
	q.Set("number", a.Number)
	q.Set("district", a.District)
	q.Set("city", a.City)
	q.Set("state", a.State)
	q.Set("zip", a.Zip)
	var out struct {
		Lat *float64 `json:"lat"`
		Lng *float64 `json:"lng"`
	}
	if err := c.getJSON(ctx, "/geo?"+q.Encode(), &out); err != nil {
		return 0, 0, err
	}
	if out.Lat == nil || out.Lng == nil {
		return 0, 0, domain.ErrNotFound
	}
	return *out.Lat, *out.Lng, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
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
	if resp.StatusCode == http.StatusNotFound {
		return domain.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
