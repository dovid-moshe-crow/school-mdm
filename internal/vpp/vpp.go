// Package vpp talks to Apple Apps & Books (VPP) content-token APIs.
package vpp

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

const defaultBase = "https://vpp.itunes.apple.com/mdm/v2"

// Client associates licenses using an ASM content token.
type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) base() string {
	if strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return defaultBase
}

// AssociateLicense assigns an Adam ID license to a device serial number.
func (c *Client) AssociateLicense(ctx context.Context, adamID, serial string) error {
	token := strings.TrimSpace(c.Token)
	adamID = strings.TrimSpace(adamID)
	serial = strings.TrimSpace(serial)
	if token == "" {
		return fmt.Errorf("vpp content token required")
	}
	if adamID == "" || serial == "" {
		return fmt.Errorf("adamId and serialNumber required")
	}
	body := map[string]any{
		"assets": []map[string]string{
			{"adamId": adamID, "pricingParam": "STDQ"},
		},
		"serialNumbers": []string{serial},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/assets/associate", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("vpp associate http %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// TokenString normalizes stored token bytes (raw string or file contents).
func TokenString(raw []byte) string {
	return strings.TrimSpace(string(raw))
}
