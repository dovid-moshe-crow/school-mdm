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
		out = append(out, a.enrichRequest(r, req))
	}
	sort.Slice(out, func(i, j int) bool {
		// Admin replied last → surface first on the device portal
		ai := out[i].LastMessage != nil && out[i].LastMessage.AuthorRole == store.AuthorAdmin
		aj := out[j].LastMessage != nil && out[j].LastMessage.AuthorRole == store.AuthorAdmin
		if ai != aj {
			return ai
		}
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

// accessIndex preloads allowlist + request state once for annotating many apps.
type accessIndex struct {
	allowed map[string]struct{}
	pending map[string]struct{}
	denied  map[string]struct{}
}

func (a *API) buildAccessIndex(r *http.Request, enrollment string) (*accessIndex, error) {
	idx := &accessIndex{
		allowed: map[string]struct{}{},
		pending: map[string]struct{}{},
		denied:  map[string]struct{}{},
	}
	apps, _, err := a.Service.EffectiveAllowlist(r.Context(), enrollment)
	if err != nil {
		return nil, err
	}
	for _, v := range apps {
		idx.allowed[v] = struct{}{}
	}
	reqs, err := a.Store.ListRequests(r.Context(), nil)
	if err != nil {
		return nil, err
	}
	for _, req := range reqs {
		if req.EnrollmentID != enrollment || req.Type != store.TypeAccess || req.TargetKind != policy.KindApp {
			continue
		}
		v := policy.Normalize(policy.KindApp, req.Value)
		if req.Status == store.StatusPending {
			idx.pending[v] = struct{}{}
		} else if req.Status == store.StatusDenied {
			idx.denied[v] = struct{}{}
		}
	}
	return idx, nil
}

func (idx *accessIndex) status(bundleID string) string {
	v := policy.Normalize(policy.KindApp, bundleID)
	if _, ok := idx.allowed[v]; ok {
		return "allowed"
	}
	if _, ok := idx.pending[v]; ok {
		return "pending"
	}
	if _, ok := idx.denied[v]; ok {
		return "denied"
	}
	return "none"
}

func (a *API) annotateApps(r *http.Request, list []store.AppMeta, enrollment string) []map[string]any {
	var idx *accessIndex
	if enrollment != "" {
		idx, _ = a.buildAccessIndex(r, enrollment)
	}
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		row := appMetaJSON(m)
		if idx != nil {
			row["access_status"] = idx.status(m.BundleID)
		}
		out = append(out, row)
	}
	return out
}

func appMetaJSON(m store.AppMeta) map[string]any {
	row := map[string]any{
		"bundle_id":   m.BundleID,
		"track_id":    m.TrackID,
		"app_name":    m.Name,
		"developer":   m.Artist,
		"artwork_url": m.ArtworkURL,
		"store_url":   m.StoreURL,
		"source":      m.Source,
	}
	if m.Description != "" {
		row["description"] = m.Description
	}
	if m.Genre != "" {
		row["genre"] = m.Genre
	}
	if m.Version != "" {
		row["version"] = m.Version
	}
	if m.AverageRating > 0 {
		row["average_rating"] = m.AverageRating
	}
	if m.RatingCount > 0 {
		row["rating_count"] = m.RatingCount
	}
	if m.ContentRating != "" {
		row["content_rating"] = m.ContentRating
	}
	if m.ReleaseDate != "" {
		row["release_date"] = m.ReleaseDate
	}
	if m.FormattedPrice != "" {
		row["formatted_price"] = m.FormattedPrice
	}
	if m.FileSizeBytes > 0 {
		row["file_size_bytes"] = m.FileSizeBytes
	}
	if m.SellerName != "" {
		row["seller_name"] = m.SellerName
	}
	if len(m.Screenshots) > 0 {
		row["screenshots"] = m.Screenshots
	}
	return row
}
