package activity

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/store"
)

// Logger writes structured audit events (best-effort; never fails callers).
type Logger struct {
	Store    store.Store
	Slog     *slog.Logger
	Webhooks Dispatcher
}

// Dispatcher fans activity events out to webhook subscribers.
type Dispatcher interface {
	Dispatch(ctx context.Context, ev store.ActivityEvent)
}

// Event is a convenient builder input for Record.
type Event struct {
	Category     string
	Action       string
	ActorType    string
	Actor        string
	EnrollmentID string
	GroupID      string
	RequestID    string
	CommandUUID  string
	Result       string
	Summary      string
	Detail       any
}

// Log is an alias for Record (call sites use Log).
func (l *Logger) Log(ctx context.Context, e Event) { l.Record(ctx, e) }

// Record persists an activity event. Errors are only written to slog.
func (l *Logger) Record(ctx context.Context, e Event) {
	if l == nil || l.Store == nil {
		return
	}
	result := strings.TrimSpace(e.Result)
	if result == "" {
		result = store.ActivityResultOK
	}
	actorType := strings.TrimSpace(e.ActorType)
	if actorType == "" {
		actorType = store.ActivityActorSystem
	}
	var detail json.RawMessage
	if e.Detail != nil {
		raw, err := json.Marshal(e.Detail)
		if err != nil {
			if l.Slog != nil {
				l.Slog.Warn("activity detail marshal", "err", err)
			}
			detail = json.RawMessage(`{}`)
		} else {
			detail = raw
		}
	}
	saved, err := l.Store.InsertActivityEvent(ctx, store.ActivityEvent{
		Category:     strings.TrimSpace(e.Category),
		Action:       strings.TrimSpace(e.Action),
		ActorType:    actorType,
		Actor:        strings.TrimSpace(e.Actor),
		EnrollmentID: strings.TrimSpace(e.EnrollmentID),
		GroupID:      strings.TrimSpace(e.GroupID),
		RequestID:    strings.TrimSpace(e.RequestID),
		CommandUUID:  strings.TrimSpace(e.CommandUUID),
		Result:       result,
		Summary:      strings.TrimSpace(e.Summary),
		Detail:       detail,
	})
	if err != nil && l.Slog != nil {
		l.Slog.Warn("activity insert", "category", e.Category, "action", e.Action, "err", err)
		return
	}
	if err == nil && l.Webhooks != nil {
		l.Webhooks.Dispatch(ctx, saved)
	}
}

// AdminFingerprint returns a short non-secret label for an admin token.
func AdminFingerprint(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "admin"
	}
	if len(token) <= 4 {
		return "admin…" + token
	}
	return "admin…" + token[len(token)-4:]
}
