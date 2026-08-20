package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/micromdm/nanodep/godep"

	"github.com/dwdmsh/school-mdm/internal/abm"
	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleABMAccount(w http.ResponseWriter, r *http.Request) {
	if a.ABM == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ABM/DEP not configured"})
		return
	}
	account, err := a.ABM.Account(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (a *API) handleABMSettingsGet(w http.ResponseWriter, r *http.Request) {
	settings, err := a.Store.GetMDMSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) handleABMSettingsPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DepName            *string `json:"dep_name"`
		DEPProfileUUID     *string `json:"dep_profile_uuid"`
		CompanionBundleID  *string `json:"companion_bundle_id"`
		CompanionITunesID  *int64  `json:"companion_itunes_id"`
		CompanionEnabled   *bool   `json:"companion_enabled"`
		LockScreenEnabled  *bool   `json:"lock_screen_enabled"`
		LockScreenFootnote *string `json:"lock_screen_footnote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	settings, err := a.Store.GetMDMSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	prevLockEnabled := settings.LockScreenEnabled
	prevFootnote := settings.LockScreenFootnote
	if body.DepName != nil {
		name := strings.TrimSpace(*body.DepName)
		if err := validateDepNameAPI(name); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		settings.DepName = name
	}
	if body.DEPProfileUUID != nil {
		settings.DEPProfileUUID = strings.TrimSpace(*body.DEPProfileUUID)
	}
	if body.CompanionBundleID != nil {
		settings.CompanionBundleID = strings.TrimSpace(*body.CompanionBundleID)
	}
	if body.CompanionITunesID != nil {
		if *body.CompanionITunesID < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "companion_itunes_id invalid"})
			return
		}
		settings.CompanionITunesID = *body.CompanionITunesID
	}
	if body.CompanionEnabled != nil {
		settings.CompanionEnabled = *body.CompanionEnabled
	}
	if body.LockScreenEnabled != nil {
		settings.LockScreenEnabled = *body.LockScreenEnabled
	}
	if body.LockScreenFootnote != nil {
		fn := strings.TrimSpace(*body.LockScreenFootnote)
		if len([]rune(fn)) > 200 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lock_screen_footnote too long"})
			return
		}
		settings.LockScreenFootnote = fn
	}
	settings.VPPToken = nil // preserve existing token in Upsert
	updated, err := a.Store.UpsertMDMSettings(r.Context(), settings)
	if err != nil {
		writeErr(w, err)
		return
	}
	lockChanged := prevLockEnabled != updated.LockScreenEnabled ||
		strings.TrimSpace(prevFootnote) != strings.TrimSpace(updated.LockScreenFootnote)
	if lockChanged && a.Push != nil {
		go func() {
			_ = a.Push.ReconcileAllDevices(context.Background())
		}()
	}
	actorType, actor := a.adminActor(r)
	a.audit(r, activity.Event{
		Category: store.ActivityCategoryABM, Action: "abm_settings",
		ActorType: actorType, Actor: actor,
		Result: store.ActivityResultOK, Summary: "עודכנו הגדרות ABM/KFilter",
		Detail: map[string]any{
			"dep_name":             updated.DepName,
			"dep_profile_uuid":     updated.DEPProfileUUID,
			"companion_itunes_id":  updated.CompanionITunesID,
			"companion_bundle_id":  updated.CompanionBundleID,
			"companion_enabled":    updated.CompanionEnabled,
			"lock_screen_enabled":  updated.LockScreenEnabled,
			"lock_screen_footnote": updated.LockScreenFootnote,
			"lock_screen_repushed": lockChanged,
		},
	})
	writeJSON(w, http.StatusOK, updated)
}

// schoolDEPSkipSetupItems skips Setup Assistant panes so ADE is Wi-Fi, MDM
// consent, and Welcome. Language/region and Wi-Fi cannot be skipped on iPhone.
var schoolDEPSkipSetupItems = []string{
	"Accessibility",
	"AccessibilityAppearance",
	"ActionButton",
	"AdditionalPrivacySettings",
	"Android",
	"AppStore",
	"AppleID",
	"Appearance",
	"Biometric",
	"CameraButton",
	"DeviceToDeviceMigration",
	"Diagnostics",
	"DisplayTone",
	"EnableLockdownMode",
	"FileVault",
	"HomeButtonSensitivity",
	"Intelligence",
	"Keyboard",
	"Location",
	"MessagingActivationUsingPhoneNumber",
	"OnBoarding",
	"Passcode",
	"Payment",
	"Privacy",
	"Restore",
	"RestoreCompleted",
	"SIMSetup",
	"Safety",
	"SafetyAndHandling",
	"ScreenSaver",
	"ScreenTime",
	"Siri",
	"SoftwareUpdate",
	"SpokenLanguage",
	"TOS",
	"TVHomeScreenSync",
	"TVProviderSignIn",
	"TVRoom",
	"TapToSetup",
	"TermsOfAddress",
	"Tips",
	"Tone",
	"TrueToneDisplay",
	"UnlockWithWatch",
	"UpdateCompleted",
	"Wallpaper",
	"WatchMigration",
	"Zoom",
	"iCloudDiagnostics",
	"iCloudStorage",
	"iMessageAndFaceTime",
}

func applySchoolDEPProfileDefaults(profile *godep.ProfileJson) {
	if profile == nil {
		return
	}
	if len(profile.SkipSetupItems) == 0 {
		profile.SkipSetupItems = append([]string(nil), schoolDEPSkipSetupItems...)
	}
	if profile.DoNotUseProfileFromBackup == nil {
		t := true
		profile.DoNotUseProfileFromBackup = &t
	}
}

func validateDepNameAPI(name string) error {
	if name == "" {
		return fmt.Errorf("dep_name required")
	}
	if len(name) > 64 {
		return fmt.Errorf("dep_name too long")
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("dep_name may only contain letters, digits, - and _")
	}
	return nil
}

func (a *API) handleABMDEPNames(w http.ResponseWriter, r *http.Request) {
	if a.ABM == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ABM/DEP not configured"})
		return
	}
	names, err := a.ABM.ListDEPNames(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if names == nil {
		names = []string{}
	}
	depName, err := a.ABM.DEPName(r.Context())
	if err != nil {
		// Fall back to school settings if ABM name source fails.
		if s, sErr := a.Store.GetMDMSettings(r.Context()); sErr == nil {
			depName = s.DepName
		} else {
			writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dep_name":  depName,
		"dep_names": names,
	})
}

func (a *API) handleABMSync(w http.ResponseWriter, r *http.Request) {
	if a.ABM == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ABM/DEP not configured"})
		return
	}
	actorType, actor := a.adminActor(r)
	a.goJob("abm-sync", func(ctx context.Context) {
		a.runABMSync(ctx, actorType, actor)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

func (a *API) runABMSync(ctx context.Context, actorType, actor string) {
	devices, err := a.ABM.SyncDevices(ctx)
	if err != nil {
		a.auditCtx(ctx, activity.Event{
			Category: store.ActivityCategoryABM, Action: "abm_sync",
			ActorType: actorType, Actor: actor, Result: store.ActivityResultError,
			Summary: "סנכרון מכשירי ABM נכשל", Detail: map[string]any{"error": err.Error()},
		})
		return
	}
	profileUUID := ""
	if settings, sErr := a.Store.GetMDMSettings(ctx); sErr == nil {
		profileUUID = strings.TrimSpace(settings.DEPProfileUUID)
	}
	need := abm.SerialsNeedingProfile(devices.Devices, profileUUID)
	assigned := 0
	assignErr := ""
	const assignBatch = 1000
	for i := 0; i < len(need); i += assignBatch {
		end := i + assignBatch
		if end > len(need) {
			end = len(need)
		}
		if _, err := a.ABM.AssignProfile(ctx, profileUUID, need[i:end]); err != nil {
			assignErr = err.Error()
			a.auditCtx(ctx, activity.Event{
				Category: store.ActivityCategoryABM, Action: "abm_assign",
				ActorType: actorType, Actor: actor, Result: store.ActivityResultError,
				Summary: "שיוך פרופיל הרשמה אחרי סנכרון נכשל",
				Detail:  map[string]any{"profile_uuid": profileUUID, "devices": len(need), "assigned": assigned, "error": assignErr},
			})
			break
		}
		assigned += end - i
	}
	if assigned > 0 {
		a.auditCtx(ctx, activity.Event{
			Category: store.ActivityCategoryABM, Action: "abm_assign",
			ActorType: actorType, Actor: actor, Result: store.ActivityResultOK,
			Summary: "שויך פרופיל הרשמה למכשירים בלי פרופיל",
			Detail:  map[string]any{"profile_uuid": profileUUID, "devices": assigned},
		})
		if refreshed, rErr := a.ABM.SyncDevices(ctx); rErr == nil {
			devices = refreshed
		}
	}
	raw, err := json.Marshal(devices.Devices)
	if err != nil {
		return
	}
	if _, err := a.Store.SaveABMDeviceCache(ctx, raw); err != nil {
		return
	}
	detail := map[string]any{"devices": len(devices.Devices), "assigned": assigned, "needed": len(need)}
	if assignErr != "" {
		detail["assign_error"] = assignErr
	}
	a.auditCtx(ctx, activity.Event{
		Category: store.ActivityCategoryABM, Action: "abm_sync",
		ActorType: actorType, Actor: actor, Result: store.ActivityResultOK,
		Summary: "סונכרנו מכשירים מ-Apple", Detail: detail,
	})
}

func (a *API) handleABMListDevices(w http.ResponseWriter, r *http.Request) {
	cache, err := a.Store.GetABMDeviceCache(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	var out []any
	if len(cache.Devices) > 0 {
		_ = json.Unmarshal(cache.Devices, &out)
	}
	if out == nil {
		out = []any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"devices":   out,
		"synced_at": cache.SyncedAt,
		"cached":    true,
	})
}

func (a *API) handleABMGetProfile(w http.ResponseWriter, r *http.Request) {
	if a.ABM == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ABM/DEP not configured"})
		return
	}
	uuid := strings.TrimSpace(r.URL.Query().Get("profile_uuid"))
	if uuid == "" {
		// Use the active ADE profile saved when "Define profile" last succeeded.
		if settings, err := a.Store.GetMDMSettings(r.Context()); err == nil {
			uuid = strings.TrimSpace(settings.DEPProfileUUID)
		}
	}
	if uuid == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "no enrollment profile yet — create one in Admin → הרשמה",
		})
		return
	}
	profile, err := a.ABM.GetProfile(r.Context(), uuid)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile_uuid": uuid,
		"profile":      profile,
	})
}

func (a *API) handleABMDefineProfile(w http.ResponseWriter, r *http.Request) {
	if a.ABM == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ABM/DEP not configured"})
		return
	}
	var profile godep.ProfileJson
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if profile.ProfileName == nil || strings.TrimSpace(*profile.ProfileName) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile_name is required"})
		return
	}
	if profile.Url == nil || strings.TrimSpace(*profile.Url) == "" {
		if a.Cfg.MDMPublicURL != "" {
			u := strings.TrimRight(a.Cfg.MDMPublicURL, "/") + "/enroll/profile"
			profile.Url = &u
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
			return
		}
	}
	// Apple FLAGS_INVALID: is_mdm_removable=false is only allowed when is_supervised=true.
	// godep.ProfileJson.IsMdmRemovable is a non-pointer bool (no omitempty), so the zero
	// value encodes as false and Apple rejects a profile that omits is_supervised.
	// School ADE defaults: supervised, MDM not removable by the student.
	if profile.IsSupervised == nil {
		t := true
		profile.IsSupervised = &t
	}
	if !*profile.IsSupervised {
		// Keep Apple's rule: cannot lock MDM payload without supervision.
		profile.IsMdmRemovable = true
	} else if !profile.IsMdmRemovable {
		// Explicit false is fine when supervised (school default).
		profile.IsMdmRemovable = false
	}
	if profile.IsMandatory == nil {
		t := true
		profile.IsMandatory = &t
	}
	applySchoolDEPProfileDefaults(&profile)
	resp, err := a.ABM.DefineProfile(r.Context(), &profile)
	if err != nil {
		a.auditAdmin(r, store.ActivityCategoryABM, "abm_profile_define", "יצירת פרופיל הרשמה ב-Apple נכשלה",
			map[string]any{"error": err.Error()}, "", "")
		status := http.StatusBadGateway
		msg := err.Error()
		if strings.Contains(msg, "FLAGS_INVALID") {
			status = http.StatusBadRequest
			msg = "Apple rejected the enrollment profile flags (FLAGS_INVALID). Supervised school profiles must set is_supervised=true when MDM is not removable."
		} else if strings.Contains(msg, "400") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": msg})
		return
	}
	if resp.ProfileUuid == nil || strings.TrimSpace(*resp.ProfileUuid) == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Apple did not return a profile UUID"})
		return
	}
	settings, err := a.Store.GetMDMSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	settings.DEPProfileUUID = strings.TrimSpace(*resp.ProfileUuid)
	settings.VPPToken = nil // preserve the existing content token
	if _, err := a.Store.UpsertMDMSettings(r.Context(), settings); err != nil {
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryABM, "abm_profile_define", "נוצר פרופיל הרשמה ב-Apple",
		map[string]any{"profile_name": *profile.ProfileName, "profile_uuid": *resp.ProfileUuid}, "", "")
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleABMAssignProfile(w http.ResponseWriter, r *http.Request) {
	if a.ABM == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ABM/DEP not configured"})
		return
	}
	var req struct {
		ProfileUUID string   `json:"profile_uuid"`
		Devices     []string `json:"devices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req.ProfileUUID = strings.TrimSpace(req.ProfileUUID)
	if req.ProfileUUID == "" || len(req.Devices) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile_uuid and devices required"})
		return
	}
	resp, err := a.ABM.AssignProfile(r.Context(), req.ProfileUUID, req.Devices)
	actorType, actor := a.adminActor(r)
	if err != nil {
		a.audit(r, activity.Event{
			Category: store.ActivityCategoryABM, Action: "abm_assign",
			ActorType: actorType, Actor: actor,
			Result: store.ActivityResultError, Summary: "שיוך פרופיל ABM נכשל",
			Detail: map[string]any{"profile_uuid": req.ProfileUUID, "devices": len(req.Devices), "error": err.Error()},
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	a.audit(r, activity.Event{
		Category: store.ActivityCategoryABM, Action: "abm_assign",
		ActorType: actorType, Actor: actor,
		Result: store.ActivityResultOK, Summary: "שויך פרופיל ABM למכשירים",
		Detail: map[string]any{"profile_uuid": req.ProfileUUID, "devices": len(req.Devices)},
	})
	writeJSON(w, http.StatusOK, resp)
}
