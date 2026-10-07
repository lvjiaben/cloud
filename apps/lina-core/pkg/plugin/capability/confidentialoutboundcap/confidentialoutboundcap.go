// Package confidentialoutboundcap defines the dynamic-plugin contract for
// invocation-scoped, host-managed confidential outbound operations.
package confidentialoutboundcap

import "encoding/json"

// Service exposes only an opaque host-created context. The host keeps the
// destination, credentials, interface instance, and raw provider response out
// of the guest runtime.
type Service interface {
	// Execute performs one strictly ordered step inside a short-lived provider
	// invocation and returns only its approved JSON projection.
	Execute(contextID string, sequence uint8, payload json.RawMessage) (json.RawMessage, error)
}
