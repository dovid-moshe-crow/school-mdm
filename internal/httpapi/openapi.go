package httpapi

import (
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/webhooks"
)

func (a *API) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, OpenAPISpec())
}

func OpenAPISpec() map[string]any {
	paths := map[string]any{}
	add := func(method, path, tag, summary, desc string, admin bool, extra map[string]any) {
		item, _ := paths[path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[path] = item
		}
		op := map[string]any{
			"tags":        []string{tag},
			"summary":     summary,
			"description": desc,
			"operationId": operationID(method, path),
			"responses": map[string]any{
				"200": map[string]any{"description": "Success"},
				"400": map[string]any{"description": "Bad request", "content": jsonContent(schemaRef("Error"))},
			},
		}
		if admin {
			op["security"] = []map[string]any{{"bearerAuth": []any{}}}
			op["responses"].(map[string]any)["401"] = map[string]any{
				"description": "Admin authorization required",
				"content":     jsonContent(schemaRef("Error")),
			}
		} else {
			op["security"] = []any{}
		}
		for k, v := range extra {
			op[k] = v
		}
		item[strings.ToLower(method)] = op
	}

	jsonBody := func(schema map[string]any) map[string]any {
		return map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{"schema": schema},
			},
		}
	}
	params := func(list ...map[string]any) map[string]any {
		return map[string]any{"parameters": list}
	}
	pathParam := func(name, desc string) map[string]any {
		return map[string]any{
			"name": name, "in": "path", "required": true, "description": desc,
			"schema": map[string]any{"type": "string"},
		}
	}
	queryParam := func(name, desc string) map[string]any {
		return map[string]any{
			"name": name, "in": "query", "required": false, "description": desc,
			"schema": map[string]any{"type": "string"},
		}
	}

	add("get", "/healthz", "Meta", "Health check", "Store ping and MDM mode.", false, nil)
	add("get", "/api/openapi.json", "Meta", "OpenAPI document", "Machine-readable spec for this API.", false, nil)
	add("get", "/api/auth/config", "Auth", "Auth config", "Whether Google sign-in is enabled.", false, nil)
	add("get", "/api/auth/me", "Auth", "Current admin", "Session cookie or Bearer token.", true, nil)
	add("post", "/api/auth/logout", "Auth", "Log out", "Clears the admin session cookie.", false, nil)
	add("get", "/api/auth/google/start", "Auth", "Start Google sign-in", "Redirects to Google OAuth.", false, nil)
	add("get", "/api/auth/google/callback", "Auth", "Google OAuth callback", "Sets the session cookie and redirects to /admin.", false, nil)

	add("get", "/api/allowlist", "Policy", "Effective allowlist", "Resolved apps and URLs for an enrollment.", false,
		params(queryParam("enrollment_id", "Device enrollment id")))
	add("get", "/api/allowances", "Policy", "List allowances", "Raw allowlist entries.", true, nil)
	add("post", "/api/allowances", "Policy", "Create allowance", "Add an app or URL to a global, group, or device allowlist.", true,
		map[string]any{"requestBody": jsonBody(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":          map[string]any{"type": "string", "enum": []string{"app", "url"}},
				"value":         map[string]any{"type": "string"},
				"target_type":   map[string]any{"type": "string", "enum": []string{"global", "group", "device"}},
				"target_id":     map[string]any{"type": "string"},
				"enrollment_id": map[string]any{"type": "string"},
				"group_id":      map[string]any{"type": "string"},
			},
			"required": []string{"kind", "value"},
		})})
	add("delete", "/api/allowances", "Policy", "Delete allowance", "Remove an allowlist entry by query.", true,
		params(queryParam("id", "Allowance id"), queryParam("kind", "app or url"), queryParam("value", "Bundle id or URL")))

	add("get", "/api/packs", "Packs", "List allowlist packs", "", true, nil)
	add("post", "/api/packs", "Packs", "Create pack", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}},
			"required":   []string{"name"},
		}),
	})
	add("get", "/api/packs/{id}", "Packs", "Get pack", "Pack with items and assignments.", true, params(pathParam("id", "Pack id")))
	add("patch", "/api/packs/{id}", "Packs", "Update pack", "", true, merge(params(pathParam("id", "Pack id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}},
		}),
	}))
	add("delete", "/api/packs/{id}", "Packs", "Delete pack", "", true, params(pathParam("id", "Pack id")))
	add("post", "/api/packs/{id}/items", "Packs", "Add pack item", "", true, merge(params(pathParam("id", "Pack id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"kind": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"}},
			"required":   []string{"kind", "value"},
		}),
	}))
	add("delete", "/api/packs/{id}/items", "Packs", "Remove pack item", "", true, merge(params(pathParam("id", "Pack id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"kind": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"}},
		}),
	}))
	add("post", "/api/packs/{id}/assignments", "Packs", "Assign pack", "Assign to everyone, a group, or a device.", true, merge(params(pathParam("id", "Pack id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"target_type": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}},
			"required":   []string{"target_type"},
		}),
	}))
	add("delete", "/api/packs/{id}/assignments", "Packs", "Unassign pack", "", true, merge(params(pathParam("id", "Pack id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"target_type": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}},
		}),
	}))

	add("get", "/api/profiles", "Profiles", "List custom profiles", "Uploaded .mobileconfig files. Pass enrollment_id to list profiles that currently apply to a device.", true,
		params(queryParam("enrollment_id", "Optional device enrollment id")))
	add("post", "/api/profiles", "Profiles", "Upload custom profile", "multipart/form-data with file= (a .mobileconfig) plus optional name and description, or JSON with payload_base64.", true, map[string]any{
		"requestBody": map[string]any{
			"required": true,
			"content": map[string]any{
				"multipart/form-data": map[string]any{"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file":        map[string]any{"type": "string", "format": "binary"},
						"name":        map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
					"required": []string{"file"},
				}},
				"application/json": map[string]any{"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":           map[string]any{"type": "string"},
						"description":    map[string]any{"type": "string"},
						"filename":       map[string]any{"type": "string"},
						"payload_base64": map[string]any{"type": "string"},
					},
					"required": []string{"payload_base64"},
				}},
			},
		},
	})
	add("get", "/api/profiles/{id}", "Profiles", "Get custom profile", "Metadata and assignments (payload is not included).", true, params(pathParam("id", "Profile id")))
	add("patch", "/api/profiles/{id}", "Profiles", "Rename custom profile", "", true, merge(params(pathParam("id", "Profile id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}},
		}),
	}))
	add("delete", "/api/profiles/{id}", "Profiles", "Delete custom profile", "Removes the profile from assigned devices.", true, params(pathParam("id", "Profile id")))
	add("put", "/api/profiles/{id}/payload", "Profiles", "Replace profile file", "Replace the .mobileconfig. Same upload body as create.", true, params(pathParam("id", "Profile id")))
	add("get", "/api/profiles/{id}/file", "Profiles", "Download profile file", "Returns the raw .mobileconfig.", true, params(pathParam("id", "Profile id")))
	add("post", "/api/profiles/{id}/assignments", "Profiles", "Assign custom profile", "Assign to everyone, a group, or a device. Reconciles MDM InstallProfile.", true, merge(params(pathParam("id", "Profile id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_type":   map[string]any{"type": "string", "enum": []string{"global", "group", "device"}},
				"target_id":     map[string]any{"type": "string"},
				"scope":         map[string]any{"type": "string"},
				"group_id":      map[string]any{"type": "string"},
				"enrollment_id": map[string]any{"type": "string"},
			},
		}),
	}))
	add("delete", "/api/profiles/{id}/assignments", "Profiles", "Unassign custom profile", "", true, params(
		pathParam("id", "Profile id"),
		queryParam("target_type", "global, group, or device"),
		queryParam("target_id", "Group id or enrollment id"),
	))

	add("get", "/api/groups", "Groups", "List groups", "", true, nil)
	add("post", "/api/groups", "Groups", "Create group", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}},
			"required":   []string{"name"},
		}),
	})
	add("get", "/api/groups/{id}", "Groups", "Get group", "", true, params(pathParam("id", "Group id")))
	add("patch", "/api/groups/{id}", "Groups", "Update group", "", true, merge(params(pathParam("id", "Group id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}},
		}),
	}))
	add("delete", "/api/groups/{id}", "Groups", "Delete group", "", true, params(pathParam("id", "Group id")))
	add("get", "/api/groups/{id}/members", "Groups", "List group members", "", true, params(pathParam("id", "Group id")))
	add("put", "/api/groups/{id}/members", "Groups", "Replace group members", "Full membership replace.", true, merge(params(pathParam("id", "Group id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type":       "object",
			"properties": map[string]any{"enrollment_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
			"required":   []string{"enrollment_ids"},
		}),
	}))

	add("get", "/api/devices", "Devices", "List school devices", "Enrollment names, groups, unrestricted flag.", true, nil)
	add("patch", "/api/devices/{id}", "Devices", "Update device", "Rename or toggle unrestricted.", true, merge(params(pathParam("id", "Enrollment id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string"},
				"unrestricted": map[string]any{"type": "boolean"},
			},
		}),
	}))

	add("get", "/api/apps/search", "Apps", "Search App Store", "", false, params(queryParam("q", "Search text"), queryParam("enrollment_id", "Optional enrollment for access status")))
	add("get", "/api/apps/lookup", "Apps", "Look up many apps", "Resolves friendly names and icons for bundle IDs (cache, known system apps, App Store).", false,
		params(queryParam("id", "Repeat for each bundle id"), queryParam("ids", "Comma-separated bundle ids"), queryParam("fetch", "0 to skip App Store")))
	add("get", "/api/apps/{bundleID}", "Apps", "Look up app", "", false, merge(params(pathParam("bundleID", "iOS bundle id"), queryParam("full", "1 for a full lookup"), queryParam("enrollment_id", "Optional enrollment")), nil))
	add("get", "/api/access-status", "Apps", "Access status", "allowed, pending, denied, or none.", false,
		params(queryParam("enrollment_id", "Enrollment id"), queryParam("kind", "app or url"), queryParam("value", "Bundle id or URL")))

	add("post", "/api/requests", "Requests", "Create request", "Student access / support ticket.", false, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("get", "/api/requests", "Requests", "List requests", "Admin queue.", true, params(
		queryParam("status", "open, approved, denied"),
		queryParam("enrollment_id", "Filter by device"),
		queryParam("q", "Search"),
	))
	add("get", "/api/requests/{id}", "Requests", "Get request", "", true, params(pathParam("id", "Request id")))
	add("get", "/api/requests/{id}/messages", "Requests", "List messages", "", false, params(pathParam("id", "Request id"), queryParam("enrollment_id", "Student view")))
	add("post", "/api/requests/{id}/messages", "Requests", "Admin reply", "", true, merge(params(pathParam("id", "Request id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type": "object", "properties": map[string]any{"body": map[string]any{"type": "string"}}, "required": []string{"body"},
		}),
	}))
	add("post", "/api/requests/{id}/approve", "Requests", "Approve request", "", true, merge(params(pathParam("id", "Request id")), map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	}))
	add("post", "/api/requests/{id}/deny", "Requests", "Deny request", "", true, merge(params(pathParam("id", "Request id")), map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	}))
	add("get", "/api/device/{deviceID}/requests", "Requests", "Device request list", "Student portal.", false, params(pathParam("deviceID", "Enrollment id")))
	add("post", "/api/device/{deviceID}/requests/{id}/messages", "Requests", "Student reply", "", false, merge(
		params(pathParam("deviceID", "Enrollment id"), pathParam("id", "Request id")),
		map[string]any{"requestBody": jsonBody(map[string]any{
			"type": "object", "properties": map[string]any{"body": map[string]any{"type": "string"}}, "required": []string{"body"},
		})},
	))
	add("post", "/api/device/{deviceID}/push-token", "Devices", "Register push token", "Companion APNs token.", false, params(pathParam("deviceID", "Enrollment id")))
	add("post", "/api/devices/{id}/push-token", "Devices", "Register push token (alias)", "", false, params(pathParam("id", "Enrollment id")))

	add("get", "/api/credits/balance", "Credits", "Credit balance", "", false, params(queryParam("enrollment_id", "Enrollment id")))
	add("get", "/api/credits/packages", "Credits", "Public packages", "", false, nil)
	add("get", "/api/credits/settings", "Credits", "Public credit settings", "", false, nil)
	add("post", "/api/credits/checkout", "Credits", "Start checkout", "", false, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("post", "/api/credits/confirm", "Credits", "Confirm checkout", "", false, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("post", "/api/admin/credits/gift", "Credits", "Gift credits", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("post", "/api/admin/credits/adjust", "Credits", "Adjust credits", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("get", "/api/admin/credits", "Credits", "List balances", "", true, nil)
	add("get", "/api/admin/credits/ledger", "Credits", "Credit ledger", "", true, params(queryParam("enrollment_id", "Filter")))
	add("get", "/api/admin/credits/purchases", "Credits", "List purchases", "", true, nil)
	add("get", "/api/admin/credits/settings", "Credits", "Admin credit settings", "", true, nil)
	add("put", "/api/admin/credits/settings", "Credits", "Update credit settings", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("get", "/api/admin/credits/packages", "Credits", "Admin packages", "", true, nil)
	add("post", "/api/admin/credits/packages", "Credits", "Create package", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("put", "/api/admin/credits/packages/{id}", "Credits", "Update package", "", true, params(pathParam("id", "Package id")))
	add("delete", "/api/admin/credits/packages/{id}", "Credits", "Delete package", "", true, params(pathParam("id", "Package id")))
	add("get", "/api/admin/credits/allotments", "Credits", "List allotment rules", "", true, nil)
	add("post", "/api/admin/credits/allotments", "Credits", "Create allotment rule", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("put", "/api/admin/credits/allotments/{id}", "Credits", "Update allotment rule", "", true, params(pathParam("id", "Rule id")))
	add("delete", "/api/admin/credits/allotments/{id}", "Credits", "Delete allotment rule", "", true, params(pathParam("id", "Rule id")))
	add("post", "/api/admin/credits/allotments/run", "Credits", "Run allotments now", "", true, nil)

	add("get", "/api/admin/activity", "Activity", "List activity events", "Audit log that also drives outbound webhooks.", true, params(
		queryParam("category", "mdm, policy, groups, requests, credits, devices, abm, system, webhooks"),
		queryParam("action", "Exact action"),
		queryParam("enrollment_id", "Device filter"),
		queryParam("actor_type", "admin, device, system, webhook"),
		queryParam("result", "ok, error, info"),
		queryParam("q", "Summary search"),
		queryParam("from", "RFC3339"),
		queryParam("to", "RFC3339"),
		queryParam("limit", "Default 50"),
		queryParam("offset", "Pagination"),
	))

	mdmDevice := func(method, suffix, summary, desc string, body map[string]any) {
		extra := params(pathParam("id", "MDM enrollment id"))
		if body != nil {
			extra = merge(extra, map[string]any{"requestBody": jsonBody(body)})
		}
		add(method, "/api/mdm/devices/{id}"+suffix, "MDM", summary, desc, true, extra)
	}
	add("get", "/api/mdm/status", "MDM", "MDM status", "Live vs stub, topic, push cert.", true, nil)
	add("get", "/api/mdm/devices", "MDM", "List MDM enrollments", "", true, nil)
	add("get", "/api/mdm/devices/{id}", "MDM", "Get MDM device", "", true, params(pathParam("id", "Enrollment id")))
	add("delete", "/api/mdm/devices/{id}", "MDM", "Delete MDM enrollment", "", true, params(pathParam("id", "Enrollment id")))
	mdmDevice("post", "/push", "Push device", "Wake the device over APNs.", nil)
	mdmDevice("post", "/install-profile", "Install profile", "Raw mobileconfig body.", map[string]any{"type": "object", "additionalProperties": true})
	mdmDevice("post", "/remove-profile", "Remove profile", "", map[string]any{
		"type": "object", "properties": map[string]any{"identifier": map[string]any{"type": "string"}},
	})
	mdmDevice("post", "/device-information", "Device information", "Queues DeviceInformation; returns command_uuid.", nil)
	add("get", "/api/mdm/devices/{id}/commands/{commandUUID}", "MDM", "Command result", "Poll a previously queued command.", true,
		params(pathParam("id", "Enrollment id"), pathParam("commandUUID", "Command UUID")))
	mdmDevice("post", "/profile-list", "Profile list", "", nil)
	mdmDevice("post", "/installed-apps", "Installed apps", "", nil)
	mdmDevice("post", "/reconcile", "Reconcile policy", "Push current allowlist + web clip.", nil)
	mdmDevice("post", "/install-companion", "Install companion", "VPP install of KFilter.", nil)
	mdmDevice("post", "/configure-companion", "Configure companion", "Managed App Config.", nil)
	mdmDevice("post", "/clear-allowlist", "Clear allowlist profile", "", nil)
	mdmDevice("post", "/lock", "Lock device", "", map[string]any{
		"type":       "object",
		"properties": map[string]any{"pin": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}},
	})
	mdmDevice("post", "/clear-passcode", "Clear passcode", "", nil)
	mdmDevice("post", "/restart", "Restart device", "", nil)
	mdmDevice("post", "/shutdown", "Shut down device", "", nil)
	mdmDevice("post", "/erase", "Erase device", "", map[string]any{"type": "object", "additionalProperties": true})
	mdmDevice("post", "/lost-mode/enable", "Enable Lost Mode", "", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message":  map[string]any{"type": "string"},
			"phone":    map[string]any{"type": "string"},
			"footnote": map[string]any{"type": "string"},
		},
	})
	mdmDevice("post", "/lost-mode/disable", "Disable Lost Mode", "", nil)
	mdmDevice("post", "/lost-mode/play-sound", "Play Lost Mode sound", "", nil)
	mdmDevice("post", "/lost-mode/location", "Lost Mode location", "", nil)
	mdmDevice("post", "/security-info", "Security info", "", nil)
	add("post", "/api/mdm/devices/bulk", "MDM", "Bulk device actions", "unrestricted, restrict, lock, clear-passcode, restart, shutdown, erase, lost mode, add-group.", true, map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"enrollment_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"op":             map[string]any{"type": "string"},
				"pin":            map[string]any{"type": "string"},
				"message":        map[string]any{"type": "string"},
				"phone":          map[string]any{"type": "string"},
				"footnote":       map[string]any{"type": "string"},
				"group_id":       map[string]any{"type": "string"},
			},
			"required": []string{"enrollment_ids", "op"},
		}),
	})
	add("put", "/api/mdm/pushcert", "MDM", "Upload APNs push cert", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})

	add("get", "/api/mdm/abm/account", "ABM", "ABM account", "", true, nil)
	add("get", "/api/mdm/abm/settings", "ABM", "ADE / companion settings", "", true, nil)
	add("put", "/api/mdm/abm/settings", "ABM", "Save ADE / companion settings", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("put", "/api/mdm/vpp/token", "ABM", "Upload VPP token", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("delete", "/api/mdm/vpp/token", "ABM", "Delete VPP token", "", true, nil)
	add("get", "/api/mdm/abm/dep-names", "ABM", "DEP configuration names", "", true, nil)
	add("get", "/api/mdm/abm/devices", "ABM", "Cached ABM devices", "", true, nil)
	add("post", "/api/mdm/abm/sync", "ABM", "Sync ABM devices", "Fetches the Apple device list and assigns the school DEP enrollment profile to devices that do not have it yet.", true, nil)
	add("get", "/api/mdm/abm/profile", "ABM", "Get DEP profile", "", true, nil)
	add("post", "/api/mdm/abm/profile", "ABM", "Define DEP profile", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})
	add("post", "/api/mdm/abm/assign", "ABM", "Assign DEP profile", "", true, map[string]any{
		"requestBody": jsonBody(map[string]any{"type": "object", "additionalProperties": true}),
	})

	add("get", "/api/webhooks/events", "Webhooks", "Event catalog", "Names you can subscribe to, plus * and category.* filters.", true, nil)
	add("get", "/api/webhooks", "Webhooks", "List webhook endpoints", "", true, nil)
	add("post", "/api/webhooks", "Webhooks", "Create webhook endpoint", "HTTPS required (http only for localhost). Empty events defaults to *.", true, map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":         map[string]any{"type": "string", "format": "uri"},
				"secret":      map[string]any{"type": "string", "description": "HMAC secret; generated if omitted"},
				"description": map[string]any{"type": "string"},
				"events":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "example": []string{"*"}},
				"enabled":     map[string]any{"type": "boolean"},
			},
			"required": []string{"url"},
		}),
	})
	add("get", "/api/webhooks/{id}", "Webhooks", "Get webhook endpoint", "", true, params(pathParam("id", "Endpoint id")))
	add("patch", "/api/webhooks/{id}", "Webhooks", "Update webhook endpoint", "", true, merge(params(pathParam("id", "Endpoint id")), map[string]any{
		"requestBody": jsonBody(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":         map[string]any{"type": "string"},
				"secret":      map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"events":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"enabled":     map[string]any{"type": "boolean"},
			},
		}),
	}))
	add("delete", "/api/webhooks/{id}", "Webhooks", "Delete webhook endpoint", "", true, params(pathParam("id", "Endpoint id")))
	add("get", "/api/webhooks/{id}/deliveries", "Webhooks", "Delivery attempts", "", true, merge(
		params(pathParam("id", "Endpoint id"), queryParam("limit", "Default 50")),
		nil,
	))
	add("post", "/api/webhooks/{id}/test", "Webhooks", "Send test ping", "Posts webhooks.ping immediately; does not write an activity row.", true, params(pathParam("id", "Endpoint id")))

	timerBody := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"action":      map[string]any{"type": "string", "enum": []string{"add", "remove"}},
			"pack_ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"profile_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Custom uploaded configuration profiles"},
			"device_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"group_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"schedule":    map[string]any{"type": "string", "enum": []string{"once", "weekly"}},
			"run_at":      map[string]any{"type": "string", "format": "date-time", "description": "UTC instant for one-shot timers"},
			"weekdays":    map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 0, "maximum": 6}, "description": "0=Sunday … 6=Saturday (Israel week)"},
			"time_of_day": map[string]any{"type": "string", "example": "08:00", "description": "HH:MM in Asia/Jerusalem"},
			"enabled":     map[string]any{"type": "boolean"},
		},
	}
	add("get", "/api/timers", "Timers", "List policy timers", "Scheduled add/remove of whitelist packs and custom profiles. Weekly clocks use Asia/Jerusalem.", true, nil)
	add("post", "/api/timers", "Timers", "Create policy timer", "Add or remove packs and/or custom profiles on devices and/or groups at a time.", true,
		map[string]any{"requestBody": jsonBody(timerBody)})
	add("post", "/api/timers/run-due", "Timers", "Run due timers", "Fires every timer whose schedule is due now (also runs every minute in the server).", true, nil)
	add("get", "/api/timers/{id}", "Timers", "Get policy timer", "", true, params(pathParam("id", "Timer id")))
	add("patch", "/api/timers/{id}", "Timers", "Update policy timer", "Changing the schedule clears last-run so it can fire again.", true, merge(
		params(pathParam("id", "Timer id")),
		map[string]any{"requestBody": jsonBody(timerBody)},
	))
	add("delete", "/api/timers/{id}", "Timers", "Delete policy timer", "", true, params(pathParam("id", "Timer id")))
	add("post", "/api/timers/{id}/run", "Timers", "Run timer now", "Applies add/remove immediately without consuming the scheduled occurrence.", true, params(pathParam("id", "Timer id")))

	add("get", "/api/stub-commands", "Meta", "Stub MDM commands", "Only populated when MDM is not live.", true, nil)

	eventItems := make([]map[string]any, 0, len(webhooks.Catalog()))
	for _, e := range webhooks.Catalog() {
		eventItems = append(eventItems, map[string]any{
			"name": e.Name, "category": e.Category, "action": e.Action, "description": e.Description,
		})
	}

	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "School MDM API",
			"version": "1.0.0",
			"description": strings.TrimSpace(`
HTTP API for every admin-panel capability: devices, groups, allowlists, packs, custom profiles, timers, requests, credits, Apple MDM, ABM/ADE, activity, and outbound webhooks.

## Auth
Admin UI uses **Sign in with Google** (session cookie). Scripts can send ` + "`Authorization: Bearer <ADMIN_TOKENS>`" + ` or HTTP Basic (password = admin token). Locked routes require one of those.

Student portal routes (` + "`/api/device/...`" + `, create request, app search, credits checkout) stay unauthenticated and are scoped by enrollment id.

## Webhooks
Each activity event is posted to matching endpoints as JSON:

` + "```json\n{\n  \"event\": \"requests.request_approve\",\n  \"id\": \"…\",\n  \"created_at\": \"2026-08-18T00:00:00Z\",\n  \"data\": { \"category\": \"requests\", \"action\": \"request_approve\", \"…\": \"…\" }\n}\n```" + `

Headers: ` + "`X-SchoolMDM-Event`" + `, ` + "`X-SchoolMDM-Delivery`" + `, ` + "`X-SchoolMDM-Event-Id`" + `, ` + "`X-SchoolMDM-Signature: sha256=<hex>`" + ` (HMAC-SHA256 of the raw body using the endpoint secret).

Subscribe with ` + "`*`" + `, ` + "`mdm.*`" + `, or an exact name from ` + "`GET /api/webhooks/events`" + `. HTTPS is required except for localhost. Failed deliveries retry three times.
`),
		},
		"servers": []map[string]any{{"url": "/", "description": "This School MDM host"}},
		"tags": []map[string]any{
			{"name": "Meta", "description": "Health and OpenAPI"},
			{"name": "Auth", "description": "Admin Google sign-in and session"},
			{"name": "Devices", "description": "School device records"},
			{"name": "Groups", "description": "Device groups"},
			{"name": "Policy", "description": "Allowlists"},
			{"name": "Packs", "description": "Reusable allowlist packs"},
			{"name": "Profiles", "description": "Uploaded Apple configuration profiles"},
			{"name": "Timers", "description": "Scheduled add/remove of packs and custom profiles on devices and groups"},
			{"name": "Apps", "description": "App Store catalog"},
			{"name": "Requests", "description": "Student tickets"},
			{"name": "Credits", "description": "Balances, packages, allotments"},
			{"name": "Activity", "description": "Audit log"},
			{"name": "MDM", "description": "Apple MDM commands"},
			{"name": "ABM", "description": "Apple Business Manager / ADE"},
			{"name": "Webhooks", "description": "Outbound activity webhooks"},
		},
		"paths": paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type": "http", "scheme": "bearer", "bearerFormat": "Admin token",
					"description": "ADMIN_TOKENS for scripts, or a Google session cookie from the admin UI.",
				},
			},
			"schemas": map[string]any{
				"Error": map[string]any{
					"type":       "object",
					"properties": map[string]any{"error": map[string]any{"type": "string"}},
				},
				"WebhookEnvelope": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"event":      map[string]any{"type": "string", "example": "mdm.lock"},
						"id":         map[string]any{"type": "string"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
						"data":       map[string]any{"$ref": "#/components/schemas/ActivityEvent"},
					},
				},
				"ActivityEvent": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":            map[string]any{"type": "string"},
						"at":            map[string]any{"type": "string", "format": "date-time"},
						"category":      map[string]any{"type": "string"},
						"action":        map[string]any{"type": "string"},
						"actor_type":    map[string]any{"type": "string"},
						"actor":         map[string]any{"type": "string"},
						"enrollment_id": map[string]any{"type": "string"},
						"group_id":      map[string]any{"type": "string"},
						"request_id":    map[string]any{"type": "string"},
						"command_uuid":  map[string]any{"type": "string"},
						"result":        map[string]any{"type": "string"},
						"summary":       map[string]any{"type": "string"},
						"detail":        map[string]any{"type": "object"},
					},
				},
			},
			"x-webhook-events": eventItems,
		},
	}
}

func jsonContent(schema map[string]any) map[string]any {
	return map[string]any{"application/json": map[string]any{"schema": schema}}
}

func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func operationID(method, path string) string {
	p := strings.Trim(path, "/")
	p = strings.ReplaceAll(p, "/", "_")
	p = strings.ReplaceAll(p, "{", "")
	p = strings.ReplaceAll(p, "}", "")
	p = strings.ReplaceAll(p, "-", "_")
	return strings.ToLower(method) + "_" + p
}

func merge(a map[string]any, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
