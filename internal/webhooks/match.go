package webhooks

import "strings"

// EventName is the dotted type posted to subscribers: category.action.
func EventName(category, action string) string {
	return strings.TrimSpace(category) + "." + strings.TrimSpace(action)
}

// Match reports whether event (category.action) is covered by filters.
// Filters: "*", "category.*", or an exact "category.action". Empty filters match all.
func Match(filters []string, event string) bool {
	event = strings.TrimSpace(event)
	if event == "" {
		return false
	}
	if len(filters) == 0 {
		return true
	}
	for _, raw := range filters {
		f := strings.TrimSpace(raw)
		if f == "" || f == "*" {
			return true
		}
		if f == event {
			return true
		}
		if strings.HasSuffix(f, ".*") {
			prefix := strings.TrimSuffix(f, ".*")
			if prefix != "" && strings.HasPrefix(event, prefix+".") {
				return true
			}
		}
	}
	return false
}
