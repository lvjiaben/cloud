// This file projects validated allocated IP addresses and runtime observations.
package zjmfcloud

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

func resourceObservationData(object rawObject, status string) map[string]any {
	result := map[string]any{"observed": true, "upstream_status": status}
	// Only guest connection and runtime facts are copied. In particular upstream
	// username is the hosting customer account, not the guest login user (osuser).
	for _, field := range []struct {
		source, target string
		maximum        int
	}{
		{"hostname", "hostname", 253}, {"os_name", "os_name", 128}, {"osuser", "login_username", 64},
		{"network_type", "network_type", 64}, {"bootorder", "boot_order", 128},
	} {
		var value string
		if json.Unmarshal(object[field.source], &value) == nil && safeObservationLabel(value, field.maximum) {
			result[field.target] = value
		}
	}
	for _, field := range []struct {
		source, target string
		maximum        int
	}{
		{"port", "port", 65535}, {"cpu", "cpu_cores", 65536}, {"memory", "memory_mib", 1073741824},
	} {
		if value, ok := metaInteger(object, field.source); ok && value > 0 && value <= field.maximum {
			result[field.target] = value
		}
	}
	if value, ok := metaInteger(object, "rescue"); ok && (value == 0 || value == 1) {
		result["rescue"] = value == 1
	}
	if count := normalizedIPCount(object); count != nil {
		result["ip_count"] = *count
	}
	if raw, exists := object["ip"]; exists {
		entries, err := decodeRawObjectArray(raw)
		defer clearRawObjects(entries)
		if err != nil || len(entries) > 1024 {
			return result
		}
		addresses := make([]string, 0, len(entries))
		seen := map[string]bool{}
		for _, entry := range entries {
			item, projectErr := projectIPAccessory(entry)
			if projectErr != nil {
				return result
			}
			var projected struct {
				Address string `json:"address"`
			}
			if json.Unmarshal(item.Data, &projected) != nil || projected.Address == "" {
				return result
			}
			if !seen[projected.Address] {
				addresses = append(addresses, projected.Address)
				seen[projected.Address] = true
			}
		}
		sort.Strings(addresses)
		result["addresses"] = addresses
	}
	return result
}

func safeObservationLabel(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) || strings.ContainsAny(value, "<>") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
