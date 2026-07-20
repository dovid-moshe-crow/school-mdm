package nedarim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ModeFake = "fake"
	ModeLive = "live"

	// Default iframe host used by Nedarim Plus DebitIframe.
	IframeBaseURL = "https://www.matara.pro/nedarimplus/iframe"
	// DebitIframeCreateURL registers a server-side DebitIframe transaction (live).
	DebitIframeCreateURL = "https://www.matara.pro/nedarimplus/V6/Files/WebServices/DebitIframe.aspx"
)

// Config holds Nedarim credentials and mode.
type Config struct {
	Mode        string // fake | live
	MosadID     string
	ApiPassword string
	ApiValid    string
	PortalBase  string // used to build fake iframe / webhook callback URLs
	HTTPClient  *http.Client
}

// CreateTxnInput is used to start a DebitIframe checkout.
type CreateTxnInput struct {
	AmountAgorot   int    // ILS agorot → converted to shekels for Nedarim
	ClientUniqueID string // our purchase correlation id (Param2 / ClientUniqueId)
	CallbackURL    string // webhook CallBack URL
	Comment        string
	PurchaseID     string
}

// CreateTxnResult is returned after creating (or simulating) a transaction.
type CreateTxnResult struct {
	Mode           string
	IframeURL      string
	ProviderTxID   string
	ClientUniqueID string
	// LiveParams are optional fields the parent/bridge page may post to the iframe.
	LiveParams map[string]string
}

// Client talks to Nedarim Plus (or a local fake).
type Client struct {
	Cfg Config
}

func (c *Client) mode() string {
	m := strings.ToLower(strings.TrimSpace(c.Cfg.Mode))
	if m == "" {
		return ModeFake
	}
	return m
}

func (c *Client) http() *http.Client {
	if c.Cfg.HTTPClient != nil {
		return c.Cfg.HTTPClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// CreateDebitIframe starts a checkout and returns an iframe URL the portal can embed.
func (c *Client) CreateDebitIframe(ctx context.Context, in CreateTxnInput) (CreateTxnResult, error) {
	if in.ClientUniqueID == "" {
		return CreateTxnResult{}, fmt.Errorf("client unique id is required")
	}
	if in.AmountAgorot <= 0 {
		return CreateTxnResult{}, fmt.Errorf("amount must be positive")
	}

	switch c.mode() {
	case ModeFake:
		base := strings.TrimRight(c.Cfg.PortalBase, "/")
		if base == "" {
			base = "http://localhost:8080"
		}
		u := fmt.Sprintf("%s/api/credits/fake-iframe?t=%s", base, url.QueryEscape(in.ClientUniqueID))
		return CreateTxnResult{
			Mode:           ModeFake,
			IframeURL:      u,
			ProviderTxID:   "fake-" + in.ClientUniqueID,
			ClientUniqueID: in.ClientUniqueID,
		}, nil

	case ModeLive:
		if strings.TrimSpace(c.Cfg.MosadID) == "" || strings.TrimSpace(c.Cfg.ApiValid) == "" {
			return CreateTxnResult{}, fmt.Errorf("NEDARIM_MODE=live requires NEDARIM_MOSAD_ID and NEDARIM_API_VALID")
		}
		amountShekels := fmt.Sprintf("%.2f", float64(in.AmountAgorot)/100.0)
		txID, err := c.createLiveTransaction(ctx, in, amountShekels)
		if err != nil {
			// Fall back to iframe-only flow (FinishTransaction2) when create API is unavailable.
			txID = ""
		}
		base := strings.TrimRight(c.Cfg.PortalBase, "/")
		if base == "" {
			base = "http://localhost:8080"
		}
		// Bridge page embeds the real Nedarim iframe and posts FinishTransaction2.
		iframeURL := fmt.Sprintf("%s/api/credits/nedarim-bridge?t=%s", base, url.QueryEscape(in.ClientUniqueID))
		params := map[string]string{
			"Mosad":        c.Cfg.MosadID,
			"ApiValid":     c.Cfg.ApiValid,
			"Amount":       amountShekels,
			"Currency":     "1",
			"Tashlumim":    "1",
			"PaymentType":  "Ragil",
			"Comment":      in.Comment,
			"Param2":       in.ClientUniqueID,
			"CallBack":     in.CallbackURL,
			"TransactionId": txID,
		}
		return CreateTxnResult{
			Mode:           ModeLive,
			IframeURL:      iframeURL,
			ProviderTxID:   txID,
			ClientUniqueID: in.ClientUniqueID,
			LiveParams:     params,
		}, nil

	default:
		return CreateTxnResult{}, fmt.Errorf("unknown Nedarim mode %q", c.mode())
	}
}

func (c *Client) createLiveTransaction(ctx context.Context, in CreateTxnInput, amountShekels string) (string, error) {
	form := url.Values{}
	form.Set("MosadId", c.Cfg.MosadID)
	form.Set("Mosad", c.Cfg.MosadID)
	if c.Cfg.ApiPassword != "" {
		form.Set("ApiPassword", c.Cfg.ApiPassword)
	}
	form.Set("ApiValid", c.Cfg.ApiValid)
	form.Set("Amount", amountShekels)
	form.Set("Currency", "1")
	form.Set("ClientUniqueId", in.ClientUniqueID)
	form.Set("CallbackParam", in.ClientUniqueID)
	form.Set("CallBack", in.CallbackURL)
	form.Set("Comment", in.Comment)
	form.Set("Action", "CreateTransaction")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, DebitIframeCreateURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("nedarim create status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Response may be JSON {"TransactionId":"..."} or plain text.
	var parsed struct {
		TransactionID string `json:"TransactionId"`
		TxID          string `json:"transactionId"`
		ID            string `json:"Id"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.TransactionID != "" {
			return parsed.TransactionID, nil
		}
		if parsed.TxID != "" {
			return parsed.TxID, nil
		}
		if parsed.ID != "" {
			return parsed.ID, nil
		}
	}
	text := strings.TrimSpace(string(body))
	if text != "" && !strings.Contains(text, "<") && len(text) < 128 {
		return text, nil
	}
	return "", fmt.Errorf("nedarim create: unexpected response")
}

// IframeEmbedURL is the raw Nedarim Plus PCI iframe (used by the live bridge page).
func IframeEmbedURL() string {
	return IframeBaseURL + "?language=he"
}
