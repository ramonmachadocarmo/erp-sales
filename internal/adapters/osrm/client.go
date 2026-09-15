package osrm

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

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	b := strings.TrimRight(base, "/")
	if b == "" {
		b = "https://router.project-osrm.org"
	}
	return &Client{base: b, http: &http.Client{Timeout: 45 * time.Second}}
}

func (c *Client) Table(ctx context.Context, coords []domain.Coord) (durations, distances [][]float64, err error) {
	if len(coords) < 2 {
		return nil, nil, domain.ErrInvalid
	}
	url := c.base + "/table/v1/driving/" + encodeCoords(coords) + "?annotations=duration,distance"
	var raw struct {
		Code      string      `json:"code"`
		Durations [][]float64 `json:"durations"`
		Distances [][]float64 `json:"distances"`
		Message   string      `json:"message"`
	}
	if err := c.get(ctx, url, &raw); err != nil {
		return nil, nil, err
	}
	if raw.Code != "" && raw.Code != "Ok" {
		return nil, nil, fmt.Errorf("%w: %s", domain.ErrRouting, raw.Message)
	}
	if len(raw.Durations) != len(coords) {
		return nil, nil, domain.ErrRouting
	}
	return raw.Durations, raw.Distances, nil
}

func (c *Client) Geometry(ctx context.Context, coords []domain.Coord) ([][]float64, error) {
	if len(coords) < 2 {
		return [][]float64{}, nil
	}
	url := c.base + "/route/v1/driving/" + encodeCoords(coords) + "?overview=full&geometries=geojson"
	var raw struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Routes  []struct {
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	if err := c.get(ctx, url, &raw); err != nil {
		return nil, err
	}
	if raw.Code != "" && raw.Code != "Ok" || len(raw.Routes) == 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrRouting, raw.Message)
	}
	out := make([][]float64, 0, len(raw.Routes[0].Geometry.Coordinates))
	for _, c := range raw.Routes[0].Geometry.Coordinates {
		if len(c) >= 2 {
			out = append(out, []float64{c[1], c[0]})
		}
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrRouting, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w: %s", domain.ErrRouting, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrRouting, err)
	}
	return nil
}

func encodeCoords(coords []domain.Coord) string {
	parts := make([]string, len(coords))
	for i, c := range coords {
		parts[i] = fmt.Sprintf("%f,%f", c.Lng, c.Lat)
	}
	return strings.Join(parts, ";")
}
