package stock

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"erp/services/sales-service/internal/domain"
)

type ctxKey string

const AuthHeaderKey ctxKey = "stock-authorization"

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) Product(ctx context.Context, id string) (domain.ProductLoad, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/products/"+id, nil)
	if err != nil {
		return domain.ProductLoad{}, err
	}
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.ProductLoad{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return domain.ProductLoad{}, domain.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return domain.ProductLoad{}, fmt.Errorf("stock: %s", strings.TrimSpace(string(b)))
	}
	var raw struct {
		ID       string  `json:"id"`
		WeightKg float64 `json:"weight_kg"`
		VolumeM3 float64 `json:"volume_m3"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.ProductLoad{}, err
	}
	return domain.ProductLoad{ID: raw.ID, WeightKg: raw.WeightKg, VolumeM3: raw.VolumeM3}, nil
}
