package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

type requestRow struct {
	store.Request
	App          *store.AppMeta       `json:"app,omitempty"`
	MessageCount int                  `json:"message_count,omitempty"`
	LastMessage  *store.RequestMessage `json:"last_message,omitempty"`
}

func (a *API) enrichRequest(r *http.Request, req store.Request) requestRow {
	item := requestRow{Request: req}
	if req.Type == store.TypeAccess && req.TargetKind == policy.KindApp {
		item.App = a.lookupAppMeta(r, req.Value)
	}
	if n, err := a.Store.CountRequestMessages(r.Context(), req.ID); err == nil {
		item.MessageCount = n
	}
	if last, err := a.Store.LastRequestMessage(r.Context(), req.ID); err == nil {
		item.LastMessage = &last
	}
	return item
}

type allowanceRow struct {
	Kind         string         `json:"kind"`
	Value        string         `json:"value"`
	Source       string         `json:"source"` // essential | global | group | device | grant
	TargetType   string         `json:"target_type,omitempty"`
	TargetID     string         `json:"target_id,omitempty"`
	EnrollmentID string         `json:"enrollment_id,omitempty"`
	GroupID      string         `json:"group_id,omitempty"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	App          *store.AppMeta `json:"app,omitempty"`
}

func (a *API) handleListRequests(w http.ResponseWriter, r *http.Request) {
	all, err := a.Store.ListRequests(r.Context(), nil)
	if err != nil {
		writeErr(w, err)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	typ := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	device := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if sortBy == "" {
		sortBy = "created_desc"
	}

	filtered := make([]store.Request, 0, len(all))
	for _, req := range all {
		if !matchRequestStatus(req.Status, status) {
			continue
		}
		if typ != "" && typ != "all" && string(req.Type) != typ {
			continue
		}
		if device != "" && req.EnrollmentID != device {
			continue
		}
		if q != "" && !a.requestMatchesQuery(r, req, q) {
			continue
		}
		filtered = append(filtered, req)
	}
	sortRequests(filtered, sortBy)

	out := make([]requestRow, 0, len(filtered))
	for _, req := range filtered {
		out = append(out, a.enrichRequest(r, req))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleListAllowances(w http.ResponseWriter, r *http.Request) {
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scope == "" {
		scope = "global"
	}
	kindFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	device := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	base, err := a.Store.ListAllowlist(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	grants, err := a.Store.ListGrants(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	rows := make([]allowanceRow, 0)

	switch scope {
	case "global":
		for _, e := range policy.Essentials {
			rows = append(rows, allowanceRow{Kind: "app", Value: e, Source: "essential", TargetType: "global"})
		}
		for _, e := range base {
			if e.Target.Type != policy.TargetGlobal && e.Target.Type != "" {
				continue
			}
			rows = append(rows, allowanceRow{
				Kind: string(e.Kind), Value: e.Value, Source: "global", TargetType: "global",
			})
		}
		for _, g := range grants {
			if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
				continue
			}
			if g.Target.Type != policy.TargetGlobal && g.Target.Type != "" {
				continue
			}
			rows = append(rows, allowanceRow{
				Kind: string(g.Kind), Value: g.Value, Source: "grant", TargetType: "global", ExpiresAt: g.ExpiresAt,
			})
		}

	case "group":
		if groupID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "group_id is required for scope=group"})
			return
		}
		for _, e := range base {
			if e.Target.Type == policy.TargetGroup && e.Target.ID == groupID {
				rows = append(rows, allowanceRow{
					Kind: string(e.Kind), Value: e.Value, Source: "group",
					TargetType: "group", TargetID: groupID, GroupID: groupID,
				})
			}
		}
		for _, g := range grants {
			if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
				continue
			}
			if g.Target.Type == policy.TargetGroup && g.Target.ID == groupID {
				rows = append(rows, allowanceRow{
					Kind: string(g.Kind), Value: g.Value, Source: "grant",
					TargetType: "group", TargetID: groupID, GroupID: groupID, ExpiresAt: g.ExpiresAt,
				})
			}
		}

	case "device", "effective":
		if device == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enrollment_id is required for scope=device"})
			return
		}
		groups, err := a.Store.ListGroupsForDevice(r.Context(), device)
		if err != nil {
			writeErr(w, err)
			return
		}
		apps, urls := policy.Effective(base, grants, groups, device, now)
		for _, v := range apps {
			src, tid, gid := sourceFor(policy.KindApp, v, device, groups, base, grants, now)
			rows = append(rows, allowanceRow{
				Kind: "app", Value: v, Source: src, TargetType: tid, TargetID: gid,
				EnrollmentID: device, GroupID: groupIDIf(src, gid),
				ExpiresAt: grantExpiry(policy.KindApp, v, device, groups, grants, now),
			})
		}
		for _, v := range urls {
			src, tid, gid := sourceFor(policy.KindURL, v, device, groups, base, grants, now)
			rows = append(rows, allowanceRow{
				Kind: "url", Value: v, Source: src, TargetType: tid, TargetID: gid,
				EnrollmentID: device, GroupID: groupIDIf(src, gid),
				ExpiresAt: grantExpiry(policy.KindURL, v, device, groups, grants, now),
			})
		}

	case "all":
		for _, e := range policy.Essentials {
			rows = append(rows, allowanceRow{Kind: "app", Value: e, Source: "essential", TargetType: "global"})
		}
		for _, e := range base {
			row := allowanceRow{
				Kind: string(e.Kind), Value: e.Value, Source: string(e.Target.Type),
				TargetType: string(e.Target.Type), TargetID: e.Target.ID,
			}
			if e.Target.Type == "" {
				row.Source = "global"
				row.TargetType = "global"
			}
			if e.Target.Type == policy.TargetGroup {
				row.GroupID = e.Target.ID
			}
			if e.Target.Type == policy.TargetDevice {
				row.EnrollmentID = e.Target.ID
			}
			rows = append(rows, row)
		}
		for _, g := range grants {
			if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
				continue
			}
			row := allowanceRow{
				Kind: string(g.Kind), Value: g.Value, Source: "grant",
				TargetType: string(g.Target.Type), TargetID: g.Target.ID, ExpiresAt: g.ExpiresAt,
			}
			if g.Target.Type == policy.TargetGroup {
				row.GroupID = g.Target.ID
			}
			if g.Target.Type == policy.TargetDevice {
				row.EnrollmentID = g.Target.ID
			}
			rows = append(rows, row)
		}

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be global, group, device, effective, or all"})
		return
	}

	filtered := make([]allowanceRow, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		key := row.Kind + "|" + row.Value + "|" + row.Source + "|" + row.TargetType + "|" + row.TargetID + "|" + row.EnrollmentID
		if _, ok := seen[key]; ok {
			continue
		}
		if kindFilter != "" && kindFilter != "all" && row.Kind != kindFilter {
			continue
		}
		if q != "" {
			blob := strings.ToLower(row.Value + " " + row.EnrollmentID + " " + row.GroupID + " " + row.Source + " " + row.TargetType)
			if !strings.Contains(blob, q) {
				if row.Kind == "app" {
					if meta := a.lookupAppMeta(r, row.Value); meta != nil {
						blob += " " + strings.ToLower(meta.Name+" "+meta.Artist)
					}
				}
				if !strings.Contains(blob, q) {
					continue
				}
			}
		}
		seen[key] = struct{}{}
		if row.Kind == "app" {
			row.App = a.lookupAppMeta(r, row.Value)
		}
		filtered = append(filtered, row)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Kind != filtered[j].Kind {
			return filtered[i].Kind < filtered[j].Kind
		}
		return filtered[i].Value < filtered[j].Value
	})
	writeJSON(w, http.StatusOK, filtered)
}

func (a *API) handleListDevices(w http.ResponseWriter, r *http.Request) {
	school, err := a.Store.ListDevices(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	byID := map[string]store.Device{}
	for _, d := range school {
		byID[d.EnrollmentID] = d
	}
	if a.MDMStore != nil {
		ens, err := a.MDMStore.ListEnrollments(r.Context())
		if err != nil {
			writeErr(w, err)
			return
		}
		for _, e := range ens {
			d := byID[e.ID]
			d.EnrollmentID = e.ID
			d.MDM = true
			d.SerialNumber = e.SerialNumber
			en := e.Enabled
			d.Enabled = &en
			if !e.LastSeenAt.IsZero() {
				d.LastSeenAt = e.LastSeenAt.Format(time.RFC3339)
			}
			_ = a.Store.EnsureDevice(r.Context(), e.ID)
			if meta, err := a.Store.GetDevice(r.Context(), e.ID); err == nil {
				d.Name = meta.Name
				d.Unrestricted = meta.Unrestricted
			}
			if gids, err := a.Store.ListGroupsForDevice(r.Context(), e.ID); err == nil {
				d.GroupIDs = gids
			}
			byID[e.ID] = d
		}
	}
	out := make([]store.Device, 0, len(byID))
	for _, d := range byID {
		if d.GroupIDs == nil {
			if gids, err := a.Store.ListGroupsForDevice(r.Context(), d.EnrollmentID); err == nil {
				d.GroupIDs = gids
			}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, nj := out[i].Name, out[j].Name
		if ni == "" {
			ni = out[i].EnrollmentID
		}
		if nj == "" {
			nj = out[j].EnrollmentID
		}
		return ni < nj
	})
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var body struct {
		Name         *string `json:"name"`
		Unrestricted *bool   `json:"unrestricted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := a.Store.EnsureDevice(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Name != nil {
		if err := a.Store.SetDeviceName(r.Context(), id, *body.Name); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		actorType, actor := a.adminActor(r)
		a.audit(r, activity.Event{
			Category: store.ActivityCategoryDevices, Action: "device_rename",
			ActorType: actorType, Actor: actor, EnrollmentID: id,
			Result: store.ActivityResultOK, Summary: "עודכן כינוי למכשיר",
			Detail: map[string]any{"name": *body.Name},
		})
		// Lock screen asset tag uses the display name — refresh that profile.
		if a.Push != nil {
			if err := a.Push.EnsureLockScreenMessage(r.Context(), id); err != nil && a.Log != nil {
				a.Log.Warn("lock screen after rename", "enrollment_id", id, "err", err)
			}
		}
	}
	if body.Unrestricted != nil {
		if err := a.Store.SetDeviceUnrestricted(r.Context(), id, *body.Unrestricted); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		a.auditAction(r, actionDeviceUnrestricted, map[string]any{
			"enrollment_id": id, "unrestricted": *body.Unrestricted,
		})
		if a.Notify != nil {
			a.Notify.UnrestrictedChanged(r.Context(), id, *body.Unrestricted)
		}
		if a.Push != nil {
			if err := a.Push.Reconcile(r.Context(), id); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
		}
	}
	d, err := a.Store.GetDevice(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusOK, store.Device{EnrollmentID: id})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (a *API) lookupAppMeta(r *http.Request, bundleID string) *store.AppMeta {
	if meta, err := a.Store.GetAppMeta(r.Context(), bundleID); err == nil {
		return &meta
	}
	if a.Catalog != nil {
		if meta, err := a.Catalog.LookupBundle(r.Context(), bundleID); err == nil {
			return &meta
		}
	}
	return nil
}

func matchRequestStatus(got store.RequestStatus, want string) bool {
	switch want {
	case "", "all":
		return true
	case "pending", "approved", "denied", "resolved":
		return got == store.RequestStatus(want)
	case "closed":
		return got == store.StatusApproved || got == store.StatusDenied || got == store.StatusResolved
	case "open":
		return got == store.StatusPending
	default:
		return got == store.RequestStatus(want)
	}
}

func (a *API) requestMatchesQuery(r *http.Request, req store.Request, q string) bool {
	parts := []string{
		string(req.Type),
		string(req.TargetKind),
		req.Value,
		req.EnrollmentID,
		req.Reason,
		string(req.Status),
		req.ID,
	}
	if req.TargetKind == policy.KindApp {
		if meta := a.lookupAppMeta(r, req.Value); meta != nil {
			parts = append(parts, meta.Name, meta.Artist, meta.BundleID)
		}
	}
	if d, err := a.Store.GetDevice(r.Context(), req.EnrollmentID); err == nil {
		parts = append(parts, d.Name, d.EnrollmentID)
	}
	blob := strings.ToLower(strings.Join(parts, " "))
	return strings.Contains(blob, q)
}

func sortRequests(list []store.Request, sortBy string) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		switch sortBy {
		case "created_asc":
			return a.CreatedAt.Before(b.CreatedAt)
		case "status":
			if a.Status != b.Status {
				return a.Status < b.Status
			}
			return a.CreatedAt.After(b.CreatedAt)
		case "type":
			if a.Type != b.Type {
				return a.Type < b.Type
			}
			return a.CreatedAt.After(b.CreatedAt)
		case "device":
			if a.EnrollmentID != b.EnrollmentID {
				return a.EnrollmentID < b.EnrollmentID
			}
			return a.CreatedAt.After(b.CreatedAt)
		default: // created_desc
			return a.CreatedAt.After(b.CreatedAt)
		}
	})
}

func groupIDIf(src, id string) string {
	if src == "group" {
		return id
	}
	return ""
}

// sourceFor returns source label, target type, and related id (group or device).
func sourceFor(kind policy.Kind, value, device string, groups []string, base []policy.Entry, grants []policy.Grant, now time.Time) (source, targetType, targetID string) {
	for _, e := range policy.Essentials {
		if kind == policy.KindApp && e == value {
			return "essential", "global", ""
		}
	}
	for _, e := range base {
		if e.Kind != kind || e.Value != value {
			continue
		}
		if e.Target.Applies(device, groups) {
			tt := string(e.Target.Type)
			if tt == "" {
				tt = "global"
			}
			return tt, tt, e.Target.ID
		}
	}
	for _, g := range grants {
		if g.Kind != kind || g.Value != value {
			continue
		}
		if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
			continue
		}
		if g.Target.Applies(device, groups) {
			tt := string(g.Target.Type)
			if tt == "" {
				tt = "global"
			}
			return "grant", tt, g.Target.ID
		}
	}
	return "effective", "", ""
}

func grantExpiry(kind policy.Kind, value, device string, groups []string, grants []policy.Grant, now time.Time) *time.Time {
	for _, g := range grants {
		if g.Kind != kind || g.Value != value {
			continue
		}
		if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
			continue
		}
		if g.Target.Applies(device, groups) {
			return g.ExpiresAt
		}
	}
	return nil
}
