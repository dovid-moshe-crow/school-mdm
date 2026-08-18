package webhooks

// EventInfo is one documented activity event that outbound webhooks can emit.
type EventInfo struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// Catalog is the admin-visible event list (filters accept these plus "*" and "category.*").
func Catalog() []EventInfo {
	return []EventInfo{
		{Name: "mdm.lock", Category: "mdm", Action: "lock", Description: "Device lock queued"},
		{Name: "mdm.erase", Category: "mdm", Action: "erase", Description: "Device erase queued"},
		{Name: "mdm.clear_passcode", Category: "mdm", Action: "clear_passcode", Description: "Clear passcode queued"},
		{Name: "mdm.restart", Category: "mdm", Action: "restart", Description: "Device restart queued"},
		{Name: "mdm.shutdown", Category: "mdm", Action: "shutdown", Description: "Device shutdown queued"},
		{Name: "mdm.enable_lost_mode", Category: "mdm", Action: "enable_lost_mode", Description: "Lost Mode enabled"},
		{Name: "mdm.disable_lost_mode", Category: "mdm", Action: "disable_lost_mode", Description: "Lost Mode disabled"},
		{Name: "mdm.play_lost_mode_sound", Category: "mdm", Action: "play_lost_mode_sound", Description: "Lost Mode sound queued"},
		{Name: "mdm.device_location", Category: "mdm", Action: "device_location", Description: "Lost Mode location requested"},
		{Name: "mdm.security_info", Category: "mdm", Action: "security_info", Description: "Security info requested"},
		{Name: "mdm.push", Category: "mdm", Action: "push", Description: "APNs push queued"},
		{Name: "mdm.install_profile", Category: "mdm", Action: "install_profile", Description: "Install profile queued"},
		{Name: "mdm.remove_profile", Category: "mdm", Action: "remove_profile", Description: "Remove profile queued"},
		{Name: "mdm.device_information", Category: "mdm", Action: "device_information", Description: "Device information requested"},
		{Name: "mdm.profile_list", Category: "mdm", Action: "profile_list", Description: "Profile list requested"},
		{Name: "mdm.installed_apps", Category: "mdm", Action: "installed_apps", Description: "Installed apps requested"},
		{Name: "mdm.reconcile", Category: "mdm", Action: "reconcile", Description: "Policy reconcile queued"},
		{Name: "mdm.clear_allowlist", Category: "mdm", Action: "clear_allowlist", Description: "Allowlist profile removed"},
		{Name: "mdm.install_companion", Category: "mdm", Action: "install_companion", Description: "Companion app install queued"},
		{Name: "mdm.configure_companion", Category: "mdm", Action: "configure_companion", Description: "Companion managed config queued"},
		{Name: "policy.allowlist_add", Category: "policy", Action: "allowlist_add", Description: "App or URL added to allowlist"},
		{Name: "policy.allowlist_remove", Category: "policy", Action: "allowlist_remove", Description: "App or URL removed from allowlist"},
		{Name: "policy.pack_create", Category: "policy", Action: "pack_create", Description: "Allowlist pack created"},
		{Name: "policy.pack_update", Category: "policy", Action: "pack_update", Description: "Allowlist pack updated"},
		{Name: "policy.pack_delete", Category: "policy", Action: "pack_delete", Description: "Allowlist pack deleted"},
		{Name: "policy.pack_item_add", Category: "policy", Action: "pack_item_add", Description: "Item added to pack"},
		{Name: "policy.pack_item_remove", Category: "policy", Action: "pack_item_remove", Description: "Item removed from pack"},
		{Name: "policy.pack_assign", Category: "policy", Action: "pack_assign", Description: "Pack assigned to a target"},
		{Name: "policy.pack_unassign", Category: "policy", Action: "pack_unassign", Description: "Pack assignment removed"},
		{Name: "groups.group_create", Category: "groups", Action: "group_create", Description: "Group created"},
		{Name: "groups.group_update", Category: "groups", Action: "group_update", Description: "Group updated"},
		{Name: "groups.group_delete", Category: "groups", Action: "group_delete", Description: "Group deleted"},
		{Name: "groups.group_member_add", Category: "groups", Action: "group_member_add", Description: "Device added to group"},
		{Name: "groups.group_member_remove", Category: "groups", Action: "group_member_remove", Description: "Device removed from group"},
		{Name: "requests.request_create", Category: "requests", Action: "request_create", Description: "Student access request created"},
		{Name: "requests.request_approve", Category: "requests", Action: "request_approve", Description: "Request approved"},
		{Name: "requests.request_deny", Category: "requests", Action: "request_deny", Description: "Request denied"},
		{Name: "credits.checkout_start", Category: "credits", Action: "checkout_start", Description: "Credit checkout started"},
		{Name: "credits.nedarim_webhook", Category: "credits", Action: "nedarim_webhook", Description: "Payment provider callback processed"},
		{Name: "credits.credit_gift", Category: "credits", Action: "credit_gift", Description: "Admin gifted credits"},
		{Name: "credits.credit_adjust", Category: "credits", Action: "credit_adjust", Description: "Admin adjusted credits"},
		{Name: "devices.enroll", Category: "devices", Action: "enroll", Description: "Device TokenUpdate / enrollment"},
		{Name: "devices.enroll_reconcile", Category: "devices", Action: "enroll_reconcile", Description: "Post-enroll policy push"},
		{Name: "devices.device_rename", Category: "devices", Action: "device_rename", Description: "Device display name changed"},
		{Name: "devices.device_unrestricted", Category: "devices", Action: "device_unrestricted", Description: "Unrestricted mode toggled"},
		{Name: "abm.abm_settings", Category: "abm", Action: "abm_settings", Description: "ABM / ADE settings saved"},
		{Name: "abm.abm_sync", Category: "abm", Action: "abm_sync", Description: "ABM device list synced"},
		{Name: "abm.abm_profile_define", Category: "abm", Action: "abm_profile_define", Description: "DEP enrollment profile defined"},
		{Name: "abm.abm_assign", Category: "abm", Action: "abm_assign", Description: "DEP profile assigned to devices"},
		{Name: "abm.vpp_token_upload", Category: "abm", Action: "vpp_token_upload", Description: "Apps and Books token uploaded"},
		{Name: "abm.vpp_token_delete", Category: "abm", Action: "vpp_token_delete", Description: "Apps and Books token deleted"},
		{Name: "system.reconcile_all", Category: "system", Action: "reconcile_all", Description: "Startup reconcile-all"},
		{Name: "webhooks.endpoint_create", Category: "webhooks", Action: "endpoint_create", Description: "Webhook endpoint created"},
		{Name: "webhooks.endpoint_update", Category: "webhooks", Action: "endpoint_update", Description: "Webhook endpoint updated"},
		{Name: "webhooks.endpoint_delete", Category: "webhooks", Action: "endpoint_delete", Description: "Webhook endpoint deleted"},
		{Name: "webhooks.ping", Category: "webhooks", Action: "ping", Description: "Manual test delivery (not stored as activity)"},
	}
}
