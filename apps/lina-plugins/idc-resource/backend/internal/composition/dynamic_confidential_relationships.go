// This file preserves bounded numeric relationship IDs in an explicitly
// requested sub_id map. Only the already-allowed numeric quantity limits may
// survive; dynamic dictionary keys never permit secrets or arbitrary values.
package composition

import (
	"encoding/json"
	"strconv"
)

func projectConfidentialRelationshipIDs(value any, allowed map[string]struct{}) (any, bool) {
	if array, ok := value.([]any); ok && len(array) == 0 {
		return []any{}, true
	}
	entries, ok := value.(map[string]any)
	if !ok || len(entries) > 512 {
		return nil, false
	}
	result := make(map[string]any, len(entries))
	for key, entry := range entries {
		number, err := strconv.ParseInt(key, 10, 64)
		if err != nil || number < 1 || strconv.FormatInt(number, 10) != key {
			return nil, false
		}
		projected := map[string]any{}
		if bounds, ok := entry.(map[string]any); ok {
			for _, name := range []string{"qty_minimum", "qty_maximum"} {
				if _, requested := allowed[name]; !requested {
					continue
				}
				raw, exists := bounds[name]
				if !exists {
					continue
				}
				var text string
				switch typed := raw.(type) {
				case string:
					text = typed
				case json.Number:
					text = string(typed)
				default:
					return nil, false
				}
				number, err := strconv.ParseInt(text, 10, 64)
				if err != nil || number < 0 || strconv.FormatInt(number, 10) != text {
					return nil, false
				}
				projected[name] = raw
			}
		}
		result[key] = projected
	}
	return result, true
}
