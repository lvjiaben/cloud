// This file normalizes frozen login declarations against their configured API root.
package composition

import (
	"net/url"
	"regexp"
	"strings"
)

// confidentialCanonicalLoginPath strips only a trusted declaration's versioned
// API prefix when the configured endpoint already ends in the same version.
func confidentialCanonicalLoginPath(endpoint, declared string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return declared
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return declared
	}
	version := segments[len(segments)-1]
	if !confidentialAPIVersionPattern.MatchString(version) {
		return declared
	}
	parts := strings.Split(strings.Trim(declared, "/"), "/")
	for index, part := range parts {
		if part == version && index < len(parts)-1 {
			return "/" + strings.Join(parts[index+1:], "/")
		}
	}
	return declared
}

// Versions are intentionally narrow; unrelated path prefixes cannot authorize
// broader guest endpoints through the login normalization rule.
var confidentialAPIVersionPattern = regexp.MustCompile("^[vV][0-9]+$")
