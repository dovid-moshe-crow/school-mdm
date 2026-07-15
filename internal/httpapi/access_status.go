package httpapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// handleAccessStatus reports whether an app/URL is already allowed or has a request for a device.
func (a *API) handleAccessStatus(w http.ResponseWriter, r *http.Request) {
	enrollment := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
	kind := policy.Kind(strings.TrimSpace(r.URL.Query().Get("kind")))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	if enrollment == "" || value == "" || (kind != policy.KindApp && kind != policy.KindURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enrollment_id, kind, value required"})
		return
	}
	status, err := a.accessStatus(r, enrollment, kind, value)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (a *API) handleDeviceRequests(w http.ResponseWriter, r *http.Request) {
	enrollment := strings.TrimSpace(r.PathValue("deviceID"))
	if enrollment == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device id required"})
		return
	}
	all, err := a.Store.ListRequests(r.Context(), nil)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]requestRow, 0)
	for _, req := range all {
		if req.EnrollmentID != enrollment {
			continue
		}
		item := requestRow{Request: req}
		if req.Type == store.TypeAccess && req.TargetKind == policy.KindApp {
			item.App = a.lookupAppMeta(r, req.Value)
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, out)
}

func (a *API) accessStatus(r *http.Request, enrollment string, kind policy.Kind, value string) (string, error) {
	value = policy.Normalize(kind, value)
	apps, urls, err := a.Service.EffectiveAllowlist(r.Context(), enrollment)
	if err != nil {
		return "", err
	}
	if kind == policy.KindApp {
		for _, v := range apps {
			if v == value {
				return "allowed", nil
			}
		}
	} else {
		for _, v := range urls {
			if v == value {
				return "allowed", nil
			}
		}
	}
	reqs, err := a.Store.ListRequests(r.Context(), nil)
	if err != nil {
		return "", err
	}
	denied := false
	for _, req := range reqs {
		if req.EnrollmentID != enrollment || req.Type != store.TypeAccess {
			continue
		}
		if req.TargetKind != kind || policy.Normalize(kind, req.Value) != value {
			continue
		}
		if req.Status == store.StatusPending {
			return "pending", nil
		}
		if req.Status == store.StatusDenied {
			denied = true
		}
	}
	if denied {
		return "denied", nil
	}
	return "none", nil
}

func (a *API) annotateApps(r *http.Request, list []store.AppMeta, enrollment string) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		row := map[string]any{
			"bundle_id":   m.BundleID,
			"track_id":    m.TrackID,
			"app_name":    m.Name,
			"developer":   m.Artist,
			"artwork_url": m.ArtworkURL,
			"store_url":   m.StoreURL,
		}
		if enrollment != "" {
			st, err := a.accessStatus(r, enrollment, policy.KindApp, m.BundleID)
			if err == nil {
				row["access_status"] = st
			}
		}
		out = append(out, row)
	}
	return out
}
