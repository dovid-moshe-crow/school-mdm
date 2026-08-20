package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) audit(r *http.Request, e activity.Event) {
	if r == nil {
		a.auditCtx(context.Background(), e)
		return
	}
	a.auditCtx(r.Context(), e)
}

func (a *API) auditCtx(ctx context.Context, e activity.Event) {
	if a == nil || a.Activity == nil {
		return
	}
	a.Activity.Log(ctx, e)
}

func (a *API) auditAdmin(r *http.Request, category, action, summary string, detail any, enrollmentID, groupID string) {
	actorType, actor := a.adminActor(r)
	a.audit(r, activity.Event{
		Category:     category,
		Action:       action,
		ActorType:    actorType,
		Actor:        actor,
		EnrollmentID: enrollmentID,
		GroupID:      groupID,
		Result:       store.ActivityResultOK,
		Summary:      summary,
		Detail:       detail,
	})
}

func (a *API) adminActor(r *http.Request) (actorType, actor string) {
	if u, ok := a.sessionFromRequest(r); ok && strings.TrimSpace(u.Email) != "" {
		return store.ActivityActorAdmin, u.Email
	}
	return store.ActivityActorAdmin, activity.AdminFingerprint(bearerToken(r))
}

func (a *API) handleListActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ActivityFilter{
		Category:     strings.TrimSpace(q.Get("category")),
		Action:       strings.TrimSpace(q.Get("action")),
		EnrollmentID: strings.TrimSpace(q.Get("enrollment_id")),
		ActorType:    strings.TrimSpace(q.Get("actor_type")),
		Result:       strings.TrimSpace(q.Get("result")),
		Q:            strings.TrimSpace(q.Get("q")),
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		f.Offset, _ = strconv.Atoi(v)
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	out, err := a.Store.ListActivityEvents(r.Context(), f)
	if err != nil {
		writeErr(w, err)
		return
	}
	if out == nil {
		out = []store.ActivityEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": out,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

func (a *API) auditMDM(r *http.Request, action, enrollmentID, summary string, err error, detail map[string]any) {
	actorType, actor := a.adminActor(r)
	result := store.ActivityResultOK
	if err != nil {
		result = store.ActivityResultError
		if detail == nil {
			detail = map[string]any{}
		}
		detail["error"] = err.Error()
	}
	if detail == nil {
		detail = map[string]any{}
	}
	a.audit(r, activity.Event{
		Category:     store.ActivityCategoryMDM,
		Action:       action,
		ActorType:    actorType,
		Actor:        actor,
		EnrollmentID: enrollmentID,
		Result:       result,
		Summary:      summary,
		Detail:       detail,
	})
}
