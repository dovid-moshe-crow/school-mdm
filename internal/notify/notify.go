// Package notify sends Expo push notifications to student devices.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/store"
)

const expoPushURL = "https://exp.host/--/api/v2/push/send"

// Service delivers Hebrew push notifications for portal events.
type Service struct {
	Store  store.Store
	Client *http.Client
	Log    *slog.Logger
}

func (s *Service) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// Message is one Expo push message.
type Message struct {
	EnrollmentID string
	Title        string
	Body         string
	Path         string // portal path suffix e.g. "?tab=updates"
}

// NotifyEnrollment sends a push to all tokens for one device (best-effort).
func (s *Service) NotifyEnrollment(ctx context.Context, msg Message) {
	if s == nil || s.Store == nil {
		return
	}
	id := strings.TrimSpace(msg.EnrollmentID)
	if id == "" || strings.TrimSpace(msg.Title) == "" {
		return
	}
	tokens, err := s.Store.ListPushTokens(ctx, id)
	if err != nil || len(tokens) == 0 {
		return
	}
	payloads := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		tok := strings.TrimSpace(t.Token)
		if tok == "" {
			continue
		}
		data := map[string]any{
			"enrollment_id": id,
			"path":          strings.TrimSpace(msg.Path),
		}
		payloads = append(payloads, map[string]any{
			"to":    tok,
			"title": msg.Title,
			"body":  msg.Body,
			"sound": "default",
			"data":  data,
		})
	}
	if len(payloads) == 0 {
		return
	}
	if err := s.sendExpo(ctx, payloads); err != nil {
		s.log().Warn("expo push failed", "enrollment_id", id, "err", err)
	}
}

func (s *Service) sendExpo(ctx context.Context, messages []map[string]any) error {
	body, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, expoPushURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := s.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("expo push http %d", res.StatusCode)
	}
	return nil
}

// RequestDecided notifies the student about approve/deny/resolve.
func (s *Service) RequestDecided(ctx context.Context, enrollmentID, status, value string) {
	title := "עדכון בקשה"
	label := displayValue(ctx, s.Store, value)
	body := "הבקשה שלך עודכנה"
	switch status {
	case string(store.StatusApproved):
		title = "הבקשה אושרה"
		body = "אושר: " + truncate(label, 80)
	case string(store.StatusDenied):
		title = "הבקשה נדחתה"
		body = "נדחה: " + truncate(label, 80)
	case string(store.StatusResolved):
		title = "הפנייה טופלה"
		body = truncate(label, 80)
	}
	s.NotifyEnrollment(ctx, Message{
		EnrollmentID: enrollmentID,
		Title:        title,
		Body:         body,
		Path:         "?tab=updates",
	})
}

// AdminMessage notifies the student about a new admin reply.
func (s *Service) AdminMessage(ctx context.Context, enrollmentID, preview string) {
	s.NotifyEnrollment(ctx, Message{
		EnrollmentID: enrollmentID,
		Title:        "הודעה מבית הספר",
		Body:         truncate(preview, 120),
		Path:         "?tab=updates",
	})
}

// UnrestrictedChanged notifies the student when Allow-all is toggled.
func (s *Service) UnrestrictedChanged(ctx context.Context, enrollmentID string, unrestricted bool) {
	title := "הוחזר סינון"
	body := "הרשימה המותרת חזרה לפעול במכשיר"
	if unrestricted {
		title = "הוסר סינון"
		body = "המכשיר במצב ללא הגבלות (Allow all)"
	}
	s.NotifyEnrollment(ctx, Message{
		EnrollmentID: enrollmentID,
		Title:        title,
		Body:         body,
		Path:         "?tab=store",
	})
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "…"
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func displayValue(ctx context.Context, st store.Store, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if n, ok := appmeta.KnownName(value); ok {
		return n
	}
	if st != nil {
		if meta, err := st.GetAppMeta(ctx, value); err == nil && strings.TrimSpace(meta.Name) != "" {
			return meta.Name
		}
	}
	return value
}
