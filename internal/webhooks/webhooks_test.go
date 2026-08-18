package webhooks_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
	"github.com/dwdmsh/school-mdm/internal/webhooks"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		filters []string
		event   string
		want    bool
	}{
		{nil, "mdm.lock", true},
		{[]string{"*"}, "mdm.lock", true},
		{[]string{"mdm.*"}, "mdm.lock", true},
		{[]string{"mdm.*"}, "policy.allowlist_add", false},
		{[]string{"requests.request_approve"}, "requests.request_approve", true},
		{[]string{"requests.request_approve"}, "requests.request_deny", false},
		{[]string{"mdm.lock", "credits.*"}, "credits.credit_gift", true},
		{[]string{"mdm.*"}, "mdm", false},
		{[]string{""}, "mdm.lock", true},
	}
	for _, c := range cases {
		if got := webhooks.Match(c.filters, c.event); got != c.want {
			t.Errorf("Match(%v, %q)=%v want %v", c.filters, c.event, got, c.want)
		}
	}
}

func TestDispatchPostsSignedJSON(t *testing.T) {
	var hits atomic.Int32
	var body []byte
	var eventHdr, sigHdr string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		eventHdr = r.Header.Get("X-SchoolMDM-Event")
		sigHdr = r.Header.Get("X-SchoolMDM-Signature")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mem := memory.New()
	ctx := context.Background()
	ep, err := mem.CreateWebhookEndpoint(ctx, store.WebhookEndpoint{
		URL:     srv.URL,
		Secret:  "topsecret",
		Events:  []string{"mdm.*"},
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	svc := webhooks.New(mem, nil)
	svc.Sync = true
	svc.Attempts = 1
	ev := store.ActivityEvent{
		ID:       "evt-1",
		At:       time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		Category: "mdm",
		Action:   "lock",
		Result:   "ok",
		Summary:  "locked",
	}
	svc.Dispatch(ctx, ev)

	if hits.Load() != 1 {
		t.Fatalf("hits=%d", hits.Load())
	}
	if eventHdr != "mdm.lock" {
		t.Fatalf("event header %q", eventHdr)
	}
	mac := hmac.New(sha256.New, []byte("topsecret"))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sigHdr != want {
		t.Fatalf("sig=%s want=%s", sigHdr, want)
	}
	var env webhooks.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if env.Event != "mdm.lock" || env.Data.Summary != "locked" {
		t.Fatalf("%+v", env)
	}

	dels, err := mem.ListWebhookDeliveries(ctx, ep.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dels) != 1 || dels[0].Status != store.WebhookDeliverySuccess {
		t.Fatalf("%+v", dels)
	}
}

func TestDispatchSkipsUnmatchedAndRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mem := memory.New()
	ctx := context.Background()
	_, _ = mem.CreateWebhookEndpoint(ctx, store.WebhookEndpoint{
		URL: srv.URL, Secret: "s", Events: []string{"credits.*"}, Enabled: true,
	})
	matched, _ := mem.CreateWebhookEndpoint(ctx, store.WebhookEndpoint{
		URL: srv.URL, Secret: "s", Events: []string{"*"}, Enabled: true,
	})

	svc := webhooks.New(mem, nil)
	svc.Sync = true
	svc.Attempts = 3
	svc.Backoff = time.Millisecond
	svc.Dispatch(ctx, store.ActivityEvent{ID: "1", Category: "mdm", Action: "lock"})

	if hits.Load() != 3 {
		t.Fatalf("hits=%d (unmatched endpoint should be skipped; matched retries then succeeds)", hits.Load())
	}
	dels, _ := mem.ListWebhookDeliveries(ctx, matched.ID, 20)
	if len(dels) != 3 {
		t.Fatalf("deliveries=%d", len(dels))
	}
	if dels[0].Status != store.WebhookDeliverySuccess {
		t.Fatalf("latest %+v", dels[0])
	}
}

func TestLoggerDispatchesWebhook(t *testing.T) {
	var got bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(&got, r.Body)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	mem := memory.New()
	ctx := context.Background()
	_, _ = mem.CreateWebhookEndpoint(ctx, store.WebhookEndpoint{
		URL: srv.URL, Secret: "k", Events: []string{"*"}, Enabled: true,
	})
	svc := webhooks.New(mem, nil)
	svc.Sync = true
	svc.Attempts = 1
	log := &activity.Logger{Store: mem, Webhooks: svc}
	log.Log(ctx, activity.Event{
		Category: store.ActivityCategoryRequests,
		Action:   "request_approve",
		Summary:  "approved",
	})
	if !bytes.Contains(got.Bytes(), []byte("request_approve")) {
		t.Fatalf("body=%s", got.String())
	}
}

func TestValidateURL(t *testing.T) {
	if err := webhooks.ValidateURL("https://hooks.example.com/x"); err != nil {
		t.Fatal(err)
	}
	if err := webhooks.ValidateURL("http://127.0.0.1:9/x"); err != nil {
		t.Fatal(err)
	}
	if err := webhooks.ValidateURL("http://evil.example/x"); err == nil {
		t.Fatal("expected http non-local reject")
	}
}
