// File currency_validation.go contains the strict uppercase three-letter
// currency validator shared by operations snapshots and report contracts.
package operations

import "strings"

// validCurrencyCode accepts the canonical uppercase currency representation
// used by the plugin's PostgreSQL constraints.
func validCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// normalizeCurrencyCode canonicalizes an ISO currency filter.
func normalizeCurrencyCode(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }
