// This file defines the optional Provider contract for channel-local upstream principals.

package resourcecap

import (
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// Canonical principal secret-binding names. Personally identifying profile
// fields and credentials cross the Provider boundary only through secretcap.
const (
	PrincipalBindingUsername = "username"
	PrincipalBindingPassword = "password"
	PrincipalBindingPhone    = "phone"
	PrincipalBindingEmail    = "email"
	PrincipalBindingRealName = "real_name"
)

// PrincipalLookupRequest asks an adapter to perform one exact, read-only
// username lookup. RequestID is an opaque owner correlation reference and must
// not contain the username or other customer data.
type PrincipalLookupRequest struct {
	RequestID string
	Username  secretcap.Handle
}

// PrincipalLookupResult is the normalized outcome of one exact lookup. A
// missing principal is represented only by Exists=false and an empty external
// reference; transport or decoding uncertainty must be returned as an error.
type PrincipalLookupResult struct {
	Exists               bool
	ExternalPrincipalRef string
	ObservedAt           time.Time
	Evidence             []Evidence
}

// PrincipalCreateRequest contains only opaque owner-managed secret handles.
// Username and password are required; phone, email, and real_name are optional.
type PrincipalCreateRequest struct {
	RequestID      string
	SecretBindings []SecretBinding
}

// PrincipalCreateResult proves only that one creation response carried a
// validated upstream identifier. The owner must still confirm the identifier
// with a subsequent exact LookupPrincipal before publishing a local mapping.
type PrincipalCreateResult struct {
	ExternalPrincipalRef string
	ObservedAt           time.Time
	Evidence             []Evidence
}
