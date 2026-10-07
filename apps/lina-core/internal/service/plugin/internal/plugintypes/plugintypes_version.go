// This file provides plugin ID and semantic-version validation helpers shared
// by manifest parsing, dependency checks, and runtime upgrade projections.

package plugintypes

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	// MaxPluginIDLength mirrors the runtime plugin_id column length.
	MaxPluginIDLength = 64
)

var (
	// ManifestIDPattern is the runtime-safe pattern for plugin IDs (kebab-case).
	ManifestIDPattern     = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	manifestSemverPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)
)

// ValidatePluginID validates the runtime-safe plugin ID boundary without
// inferring an author, organization, domain, or capability from its segments.
func ValidatePluginID(value string) error {
	pluginID := strings.TrimSpace(value)
	if pluginID == "" {
		return invalidPluginIDError(pluginID, "plugin ID cannot be empty")
	}
	if len(pluginID) > MaxPluginIDLength {
		return invalidPluginIDError(pluginID, "plugin ID length must not exceed 64 characters")
	}
	if !ManifestIDPattern.MatchString(pluginID) {
		return invalidPluginIDError(pluginID, "plugin ID must use kebab-case lowercase letters and digits")
	}
	return nil
}

// ValidateManifestSemanticVersion validates semantic version strings used by plugin manifests.
func ValidateManifestSemanticVersion(value string) error {
	_, err := parseSemanticVersion(value)
	return err
}

// CompareSemanticVersions compares two validated semantic-version strings.
func CompareSemanticVersions(left string, right string) (int, error) {
	leftVersion, err := parseSemanticVersion(left)
	if err != nil {
		return 0, err
	}
	rightVersion, err := parseSemanticVersion(right)
	if err != nil {
		return 0, err
	}

	switch {
	case leftVersion.Major < rightVersion.Major:
		return -1, nil
	case leftVersion.Major > rightVersion.Major:
		return 1, nil
	case leftVersion.Minor < rightVersion.Minor:
		return -1, nil
	case leftVersion.Minor > rightVersion.Minor:
		return 1, nil
	case leftVersion.Patch < rightVersion.Patch:
		return -1, nil
	case leftVersion.Patch > rightVersion.Patch:
		return 1, nil
	default:
		return 0, nil
	}
}

// parseSemanticVersion parses a manifest semantic version into comparable parts.
func parseSemanticVersion(value string) (*semanticVersion, error) {
	match := manifestSemverPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) < 4 {
		return nil, gerror.Newf("version must use semver format: %s", value)
	}

	major, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, err
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return nil, err
	}
	patch, err := strconv.Atoi(match[3])
	if err != nil {
		return nil, err
	}

	return &semanticVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// semanticVersion provides lightweight semantic-version validation for plugin manifests.
type semanticVersion struct {
	Major int
	Minor int
	Patch int
}

// invalidPluginIDError wraps plugin ID validation failures in stable runtime
// metadata for HTTP and dynamic upload call sites.
func invalidPluginIDError(pluginID string, reason string) error {
	return bizerr.NewCode(
		CodePluginIDInvalid,
		bizerr.P("pluginId", pluginID),
		bizerr.P("reason", reason),
	)
}
