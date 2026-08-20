package httpapi

import (
	"net/http"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Policy / device action names used in activity_events (audit only).
const (
	actionAllowlistAdd       = "allowlist_add"
	actionAllowlistRemove    = "allowlist_remove"
	actionGroupMemberAdd     = "group_member_add"
	actionGroupMemberRemove  = "group_member_remove"
	actionDeviceUnrestricted = "device_unrestricted"
)

// auditAction writes a policy/device mutation to the activity log.
func (a *API) auditAction(r *http.Request, action string, payload map[string]any) {
	actorType, actor := a.adminActor(r)
	enrollmentID := ""
	groupID := ""
	summary := action
	if payload != nil {
		if v, ok := payload["enrollment_id"].(string); ok {
			enrollmentID = v
		}
		if v, ok := payload["group_id"].(string); ok {
			groupID = v
		}
		switch action {
		case actionAllowlistAdd:
			summary = "נוסף לרשימת המותרים"
			if label := payloadLabel(payload); label != "" {
				summary += " · " + label
			}
		case actionAllowlistRemove:
			summary = "הוסר מרשימת המותרים"
			if label := payloadLabel(payload); label != "" {
				summary += " · " + label
			}
		case actionGroupMemberAdd:
			summary = "נוסף לקבוצה"
		case actionGroupMemberRemove:
			summary = "הוסר מקבוצה"
		case actionDeviceUnrestricted:
			if u, ok := payload["unrestricted"].(bool); ok && u {
				summary = "הופעל מצב ללא הגבלות"
			} else {
				summary = "בוטל מצב ללא הגבלות"
			}
		}
	}
	cat := store.ActivityCategoryPolicy
	switch action {
	case actionGroupMemberAdd, actionGroupMemberRemove:
		cat = store.ActivityCategoryGroups
	case actionDeviceUnrestricted:
		cat = store.ActivityCategoryDevices
	}
	a.audit(r, activity.Event{
		Category:     cat,
		Action:       action,
		ActorType:    actorType,
		Actor:        actor,
		EnrollmentID: enrollmentID,
		GroupID:      groupID,
		Result:       store.ActivityResultOK,
		Summary:      summary,
		Detail:       payload,
	})
}

func payloadLabel(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["app_name"].(string); ok && v != "" {
		return v
	}
	if v, ok := payload["value"].(string); ok {
		return v
	}
	return ""
}
