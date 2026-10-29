// This file projects guest connection facts without guessing missing values or
// leaking provider accounts, credentials, tokens or arbitrary response fields.
package customeroperations

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Information contains confirmed runtime facts; nil numbers mean unknown.
type Information struct {
	Hostname      string
	OSName        string
	LoginUsername string
	Port          *int
	CPUCores      *int
	MemoryMiB     *int
	NetworkType   string
	BootOrder     string
	Rescue        *bool
}

// Capability explains why a supported action is temporarily unavailable.
type Capability struct {
	Action               string
	Supported, Available bool
	ReasonCode           string
}

func observationInformation(raw json.RawMessage) Information {
	var fields map[string]json.RawMessage
	var result Information
	if json.Unmarshal(raw, &fields) != nil {
		return result
	}
	for _, field := range []struct {
		key     string
		target  *string
		maximum int
	}{
		{"hostname", &result.Hostname, 253}, {"os_name", &result.OSName, 128}, {"login_username", &result.LoginUsername, 64},
		{"network_type", &result.NetworkType, 64}, {"boot_order", &result.BootOrder, 128},
	} {
		var value string
		if json.Unmarshal(fields[field.key], &value) != nil || len(value) > field.maximum || value != strings.TrimSpace(value) || strings.ContainsAny(value, "<>") {
			continue
		}
		valid := true
		for _, char := range value {
			if unicode.IsControl(char) {
				valid = false
				break
			}
		}
		if valid {
			*field.target = value
		}
	}
	for _, field := range []struct {
		key     string
		target  **int
		maximum int
	}{
		{"port", &result.Port, 65535}, {"cpu_cores", &result.CPUCores, 65536}, {"memory_mib", &result.MemoryMiB, 1073741824},
	} {
		var value int
		if json.Unmarshal(fields[field.key], &value) == nil && value > 0 && value <= field.maximum {
			*field.target = &value
		}
	}
	var rescue bool
	if json.Unmarshal(fields["rescue"], &rescue) == nil && string(fields["rescue"]) != "null" {
		result.Rescue = &rescue
	}
	return result
}
