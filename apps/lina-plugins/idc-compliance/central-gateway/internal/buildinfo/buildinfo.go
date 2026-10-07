// Package buildinfo exposes immutable release metadata injected by the release tool.
package buildinfo

import "strings"

const (
	serviceName = "idc-compliance-local-gateway"
	maxValueLen = 128
)

var (
	version = "dev"
	commit  = "unknown"
	builtAt = "unknown"
)

// Info is the safe build projection returned by operational endpoints.
type Info struct {
	Service string `json:"service"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
}

// Current returns normalized build metadata without environment or secret values.
func Current() Info {
	return Info{
		Service: serviceName,
		Version: normalized(version, "dev"),
		Commit:  normalized(commit, "unknown"),
		BuiltAt: normalized(builtAt, "unknown"),
	}
}

func normalized(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if len(value) > maxValueLen {
		return value[:maxValueLen]
	}
	return value
}
