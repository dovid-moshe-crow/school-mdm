package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/profiles"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	enrollmentID := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
	if enrollmentID != "" {
		groups, err := a.Store.ListGroupsForDevice(r.Context(), enrollmentID)
		if err != nil {
			writeErr(w, err)
			return
		}
		list, err := a.Store.ListCustomProfilesForDevice(r.Context(), enrollmentID, groups)
		if err != nil {
			writeErr(w, err)
			return
		}
		out := make([]store.CustomProfile, 0, len(list))
		for _, p := range list {
			p.Payload = nil
			out = append(out, p)
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	list, err := a.Store.ListCustomProfiles(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.CustomProfile{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	payload, filename, name, description, err := readProfileUpload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := a.Store.CreateCustomProfile(r.Context(), store.CustomProfile{
		Name:        name,
		Description: description,
		Filename:    filename,
		Payload:     payload,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "profile_create", "הועלה פרופיל מותאם",
		map[string]any{"profile_id": p.ID, "name": p.Name, "payload_identifier": p.PayloadIdentifier}, "", "")
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.GetCustomProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	assigns, _ := a.Store.ListCustomProfileAssignments(r.Context(), p.ID)
	if assigns == nil {
		assigns = []store.CustomProfileAssignment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile":     p,
		"assignments": assigns,
	})
}

func (a *API) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	p, err := a.Store.UpdateCustomProfile(r.Context(), store.CustomProfile{
		ID: r.PathValue("id"), Name: body.Name, Description: body.Description,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "profile_update", "עודכן פרופיל מותאם",
		map[string]any{"profile_id": p.ID, "name": p.Name}, "", "")
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handleReplaceProfilePayload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	old, err := a.Store.GetCustomProfile(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	payload, filename, _, _, err := readProfileUpload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if filename == "" {
		filename = old.Filename
	}
	p, err := a.Store.ReplaceCustomProfilePayload(r.Context(), id, payload, filename)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	devices, _ := a.devicesAffectedByProfile(r, id)
	if old.PayloadIdentifier != p.PayloadIdentifier {
		a.removeProfileIdentifier(r, devices, old.PayloadIdentifier)
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "profile_replace", "הוחלף קובץ פרופיל מותאם",
		map[string]any{"profile_id": p.ID, "payload_identifier": p.PayloadIdentifier, "devices": len(devices)}, "", "")
	if a.Push != nil && len(devices) > 0 {
		a.pushManyLater(devices)
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handleDownloadProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := a.Store.GetCustomProfile(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	payload, err := a.Store.GetCustomProfilePayload(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	name := strings.TrimSpace(p.Filename)
	if name == "" {
		name = p.PayloadIdentifier + ".mobileconfig"
	}
	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (a *API) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := a.Store.GetCustomProfile(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	devices, _ := a.devicesAffectedByProfile(r, id)
	if err := a.Store.DeleteCustomProfile(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	a.removeProfileIdentifier(r, devices, p.PayloadIdentifier)
	a.auditAdmin(r, store.ActivityCategoryPolicy, "profile_delete", "נמחק פרופיל מותאם",
		map[string]any{"profile_id": id, "payload_identifier": p.PayloadIdentifier, "devices": len(devices)}, "", "")
	if a.Push != nil && len(devices) > 0 {
		a.pushManyLater(devices)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) handleAddProfileAssignment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Scope      string `json:"scope"`
		GroupID    string `json:"group_id"`
		Enrollment string `json:"enrollment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	tt := policy.TargetType(strings.TrimSpace(body.TargetType))
	tid := strings.TrimSpace(body.TargetID)
	if tt == "" {
		switch strings.ToLower(strings.TrimSpace(body.Scope)) {
		case "global", "":
			tt = policy.TargetGlobal
		case "group":
			tt = policy.TargetGroup
			tid = strings.TrimSpace(body.GroupID)
		case "device":
			tt = policy.TargetDevice
			tid = strings.TrimSpace(body.Enrollment)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scope"})
			return
		}
	}
	profileID := r.PathValue("id")
	if err := a.Store.SetCustomProfileAssignment(r.Context(), store.CustomProfileAssignment{
		ProfileID: profileID, TargetType: tt, TargetID: tid,
	}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	devices, _ := a.devicesForTarget(r, policy.Target{Type: tt, ID: tid})
	enrollID, groupID := "", ""
	switch tt {
	case policy.TargetDevice:
		enrollID = tid
	case policy.TargetGroup:
		groupID = tid
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "profile_assign", "שויך פרופיל מותאם",
		map[string]any{"profile_id": profileID, "target_type": string(tt), "target_id": tid, "devices": len(devices)},
		enrollID, groupID)
	if a.Push != nil && len(devices) > 0 {
		a.pushManyLater(devices)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (a *API) handleRemoveProfileAssignment(w http.ResponseWriter, r *http.Request) {
	tt := policy.TargetType(strings.TrimSpace(r.URL.Query().Get("target_type")))
	tid := strings.TrimSpace(r.URL.Query().Get("target_id"))
	profileID := r.PathValue("id")
	target := policy.Target{Type: tt, ID: tid}
	devices, _ := a.devicesForTarget(r, target)
	if err := a.Store.RemoveCustomProfileAssignment(r.Context(), profileID, target); err != nil {
		writeErr(w, err)
		return
	}
	enrollID, groupID := "", ""
	switch tt {
	case policy.TargetDevice:
		enrollID = tid
	case policy.TargetGroup:
		groupID = tid
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "profile_unassign", "בוטל שיוך פרופיל מותאם",
		map[string]any{"profile_id": profileID, "target_type": string(tt), "target_id": tid, "devices": len(devices)},
		enrollID, groupID)
	if a.Push != nil && len(devices) > 0 {
		a.pushManyLater(devices)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) devicesAffectedByProfile(r *http.Request, profileID string) ([]string, error) {
	assigns, err := a.Store.ListCustomProfileAssignments(r.Context(), profileID)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []string
	for _, as := range assigns {
		ids, err := a.devicesForTarget(r, policy.Target{Type: as.TargetType, ID: as.TargetID})
		if err != nil {
			continue
		}
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, nil
}

func (a *API) removeProfileIdentifier(r *http.Request, devices []string, identifier string) {
	if a == nil || a.Enqueue == nil || identifier == "" {
		return
	}
	for _, id := range devices {
		_ = a.Enqueue.RemoveProfile(r.Context(), id, identifier)
	}
}

func readProfileUpload(r *http.Request) (payload []byte, filename, name, description string, err error) {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/") {
		if err = r.ParseMultipartForm(int64(profiles.MaxMobileconfigBytes) + 1<<20); err != nil {
			return nil, "", "", "", fmtUpload("invalid multipart upload")
		}
		name = strings.TrimSpace(r.FormValue("name"))
		description = strings.TrimSpace(r.FormValue("description"))
		file, hdr, ferr := r.FormFile("file")
		if ferr != nil {
			return nil, "", "", "", fmtUpload("file is required")
		}
		defer file.Close()
		filename = filepath.Base(strings.TrimSpace(hdr.Filename))
		payload, err = io.ReadAll(io.LimitReader(file, int64(profiles.MaxMobileconfigBytes)+1))
		if err != nil {
			return nil, "", "", "", fmtUpload("could not read file")
		}
	} else {
		var body struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			Filename      string `json:"filename"`
			PayloadBase64 string `json:"payload_base64"`
		}
		if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, "", "", "", fmtUpload("invalid JSON")
		}
		name = strings.TrimSpace(body.Name)
		description = strings.TrimSpace(body.Description)
		filename = filepath.Base(strings.TrimSpace(body.Filename))
		raw := strings.TrimSpace(body.PayloadBase64)
		if raw == "" {
			return nil, "", "", "", fmtUpload("payload_base64 is required")
		}
		payload, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			payload, err = base64.RawStdEncoding.DecodeString(raw)
		}
		if err != nil {
			return nil, "", "", "", fmtUpload("payload_base64 is not valid base64")
		}
	}
	if len(payload) == 0 {
		return nil, "", "", "", fmtUpload("profile is empty")
	}
	if len(payload) > profiles.MaxMobileconfigBytes {
		return nil, "", "", "", fmtUpload("profile is too large")
	}
	if filename == "" {
		filename = "profile.mobileconfig"
	}
	return payload, filename, name, description, nil
}

func fmtUpload(msg string) error {
	return errors.New(msg)
}
