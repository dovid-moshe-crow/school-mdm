package httpapi

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

const accessIndexTTL = 15 * time.Second

type cachedAccessIndex struct {
	at  time.Time
	idx *accessIndex
}

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
	all, err := a.Store.ListRequestsByEnrollment(r.Context(), enrollment)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]requestRow, 0, len(all))
	for _, req := range all {
		out = append(out, a.enrichRequest(r, req))
	}
	sort.Slice(out, func(i, j int) bool {
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
	if kind == policy.KindApp {
		idx, err := a.getAccessIndex(r, enrollment)
		if err != nil {
			return "", err
		}
		return idx.status(value), nil
	}
	apps, urls, err := a.Service.EffectiveAllowlist(r.Context(), enrollment)
	if err != nil {
		return "", err
	}
	for _, v := range urls {
		if v == value {
			return "allowed", nil
		}
	}
	_ = apps
	reqs, err := a.Store.ListRequestsByEnrollment(r.Context(), enrollment)
	if err != nil {
		return "", err
	}
	denied := false
	for _, req := range reqs {
		if req.Type != store.TypeAccess || req.TargetKind != kind {
			continue
		}
		if policy.Normalize(kind, req.Value) != value {
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

func (a *API) getAccessIndex(r *http.Request, enrollment string) (*accessIndex, error) {
	enrollment = strings.TrimSpace(enrollment)
	if enrollment == "" {
		return &accessIndex{
			allowed: map[string]struct{}{},
			pending: map[string]struct{}{},
			denied:  map[string]struct{}{},
		}, nil
	}

	a.accessMu.Lock()
	if a.accessCache != nil {
		if hit, ok := a.accessCache[enrollment]; ok && time.Since(hit.at) < accessIndexTTL {
			idx := hit.idx
			a.accessMu.Unlock()
			return idx, nil
		}
	}
	a.accessMu.Unlock()

	idx, err := a.buildAccessIndex(r, enrollment)
	if err != nil {
		return nil, err
	}

	a.accessMu.Lock()
	if a.accessCache == nil {
		a.accessCache = map[string]cachedAccessIndex{}
	}
	a.accessCache[enrollment] = cachedAccessIndex{at: time.Now(), idx: idx}
	a.accessMu.Unlock()
	return idx, nil
}

func (a *API) invalidateAccessIndex(enrollment string) {
	a.accessMu.Lock()
	defer a.accessMu.Unlock()
	if a.accessCache == nil {
		return
	}
	if enrollment == "" {
		a.accessCache = map[string]cachedAccessIndex{}
		return
	}
	delete(a.accessCache, enrollment)
}

func (a *API) buildAccessIndex(r *http.Request, enrollment string) (*accessIndex, error) {
	idx := &accessIndex{
		allowed: map[string]struct{}{},
		pending: map[string]struct{}{},
		denied:  map[string]struct{}{},
	}

	var (
		apps []string
		reqs []store.Request
		err1 error
		err2 error
		wg   sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		var urls []string
		apps, urls, err1 = a.Service.EffectiveAllowlist(r.Context(), enrollment)
		_ = urls
	}()
	go func() {
		defer wg.Done()
		reqs, err2 = a.Store.ListRequestsByEnrollment(r.Context(), enrollment)
	}()
	wg.Wait()
	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	for _, v := range apps {
		idx.allowed[policy.AppKey(v)] = struct{}{}
	}
	for _, req := range reqs {
		if req.Type != store.TypeAccess || req.TargetKind != policy.KindApp {
			continue
		}
		v := policy.AppKey(req.Value)
		if req.Status == store.StatusPending {
			idx.pending[v] = struct{}{}
		} else if req.Status == store.StatusDenied {
			idx.denied[v] = struct{}{}
		}
	}
	return idx, nil
}

func (idx *accessIndex) status(bundleID string) string {
	v := policy.AppKey(bundleID)
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
		idx, _ = a.getAccessIndex(r, enrollment)
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
