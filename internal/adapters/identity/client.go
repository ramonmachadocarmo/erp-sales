package identity

import (
	"bytes"
	"context"
	"encoding/json"
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
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 10 * time.Second}}
}

// VerifyAdmin checks the given credentials against identity-service's own login — this
// approves an admin who isn't necessarily the browser's own logged-in session (see
// domain.AdminVerifier) — and reports whether they resolve to a real MASTER-role account.
// A bad password is not an error, just a "no": err is only set for a genuine failure to
// reach identity-service.
func (c *Client) VerifyAdmin(ctx context.Context, email, password string) (bool, error) {
	raw, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/auth/login", bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}
	var out struct {
		User struct {
			RoleCode string `json:"role_code"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.User.RoleCode == domain.MasterRoleCode, nil
}
