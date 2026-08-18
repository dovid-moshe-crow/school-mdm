package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/store"
)

const (
	headerSignature = "X-SchoolMDM-Signature"
	headerEvent     = "X-SchoolMDM-Event"
	headerDelivery  = "X-SchoolMDM-Delivery"
	headerEventID   = "X-SchoolMDM-Event-Id"
	userAgent       = "SchoolMDM-Webhooks/1.0"
)

// Service delivers activity events to registered HTTP endpoints.
type Service struct {
	Store   store.Store
	Log     *slog.Logger
	HTTP    *http.Client
	Now     func() time.Time
	// Sync runs Dispatch on the caller goroutine (tests).
	Sync bool
	// Attempts is POST tries per endpoint (default 3).
	Attempts int
	// Backoff is the base delay between retries (default 250ms).
	Backoff time.Duration
}

// New builds a dispatcher with a 15s HTTP timeout.
func New(st store.Store, log *slog.Logger) *Service {
	return &Service{
		Store: st,
		Log:   log,
		HTTP:  &http.Client{Timeout: 15 * time.Second},
		Now:   func() time.Time { return time.Now().UTC() },
	}
}

// Envelope is the JSON body posted to subscribers.
type Envelope struct {
	Event     string              `json:"event"`
	ID        string              `json:"id"`
	CreatedAt time.Time           `json:"created_at"`
	Data      store.ActivityEvent `json:"data"`
}

// Dispatch fans the saved activity event out to matching endpoints.
func (s *Service) Dispatch(ctx context.Context, ev store.ActivityEvent) {
	if s == nil || s.Store == nil {
		return
	}
	if s.Sync {
		s.dispatch(ctx, ev)
		return
	}
	go s.dispatch(context.WithoutCancel(ctx), ev)
}

func (s *Service) dispatch(ctx context.Context, ev store.ActivityEvent) {
	name := EventName(ev.Category, ev.Action)
	list, err := s.Store.ListWebhookEndpoints(ctx)
	if err != nil {
		s.warn("list webhook endpoints", "err", err)
		return
	}
	body, err := json.Marshal(Envelope{
		Event:     name,
		ID:        ev.ID,
		CreatedAt: ev.At,
		Data:      ev,
	})
	if err != nil {
		s.warn("marshal webhook envelope", "err", err)
		return
	}
	for _, ep := range list {
		if !ep.Enabled {
			continue
		}
		if !Match(ep.Events, name) {
			continue
		}
		s.deliver(ctx, ep, name, ev.ID, body)
	}
}

// SendTest posts a synthetic ping to one endpoint, ignoring its event filters.
func (s *Service) SendTest(ctx context.Context, ep store.WebhookEndpoint) (store.WebhookDelivery, error) {
	now := s.now()
	ev := store.ActivityEvent{
		ID:        "ping-" + uuid.NewString(),
		At:        now,
		Category:  store.ActivityCategoryWebhooks,
		Action:    "ping",
		ActorType: store.ActivityActorAdmin,
		Actor:     "api",
		Result:    store.ActivityResultInfo,
		Summary:   "Webhook test delivery",
		Detail:    json.RawMessage(`{"test":true}`),
	}
	body, err := json.Marshal(Envelope{
		Event:     EventName(ev.Category, ev.Action),
		ID:        ev.ID,
		CreatedAt: ev.At,
		Data:      ev,
	})
	if err != nil {
		return store.WebhookDelivery{}, err
	}
	return s.deliver(ctx, ep, EventName(ev.Category, ev.Action), ev.ID, body), nil
}

func (s *Service) deliver(ctx context.Context, ep store.WebhookEndpoint, eventName, eventID string, body []byte) store.WebhookDelivery {
	attempts := s.Attempts
	if attempts <= 0 {
		attempts = 3
	}
	backoff := s.Backoff
	if backoff <= 0 {
		backoff = 250 * time.Millisecond
	}

	var last store.WebhookDelivery
	for i := 1; i <= attempts; i++ {
		d := store.WebhookDelivery{
			EndpointID: ep.ID,
			EventID:    eventID,
			EventName:  eventName,
			Attempt:    i,
			Status:     store.WebhookDeliveryPending,
			CreatedAt:  s.now(),
		}
		status, httpStatus, errMsg := s.postOnce(ctx, ep, eventName, body)
		d.Status = status
		d.HTTPStatus = httpStatus
		d.Error = errMsg
		saved, err := s.Store.InsertWebhookDelivery(ctx, d)
		if err != nil {
			s.warn("insert webhook delivery", "endpoint_id", ep.ID, "err", err)
			last = d
		} else {
			last = saved
		}
		if status == store.WebhookDeliverySuccess {
			return last
		}
		if i < attempts {
			select {
			case <-ctx.Done():
				return last
			case <-time.After(backoff * time.Duration(1<<(i-1))):
			}
		}
	}
	return last
}

func (s *Service) postOnce(ctx context.Context, ep store.WebhookEndpoint, eventName string, body []byte) (status string, httpStatus int, errMsg string) {
	if err := ValidateURL(ep.URL); err != nil {
		return store.WebhookDeliveryFailed, 0, err.Error()
	}
	deliveryID := uuid.NewString()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return store.WebhookDeliveryFailed, 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(headerEvent, eventName)
	req.Header.Set(headerDelivery, deliveryID)
	req.Header.Set(headerEventID, "")
	if ev := decodeEventID(body); ev != "" {
		req.Header.Set(headerEventID, ev)
	}
	req.Header.Set(headerSignature, Sign(ep.Secret, body))

	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return store.WebhookDeliveryFailed, 0, err.Error()
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return store.WebhookDeliverySuccess, res.StatusCode, ""
	}
	return store.WebhookDeliveryFailed, res.StatusCode, fmt.Sprintf("HTTP %d", res.StatusCode)
}

func decodeEventID(body []byte) string {
	var env Envelope
	if json.Unmarshal(body, &env) != nil {
		return ""
	}
	return env.ID
}

// Sign returns the sha256=hex HMAC of body using secret.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ValidateURL requires https, or http on loopback.
func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid webhook URL")
	}
	host := strings.ToLower(u.Hostname())
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		return fmt.Errorf("webhook URL must use https (http is only allowed for localhost)")
	default:
		return fmt.Errorf("webhook URL must use https")
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) warn(msg string, args ...any) {
	if s.Log != nil {
		s.Log.Warn(msg, args...)
	}
}
