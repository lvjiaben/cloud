// This file defines source-resource confirmation without exposing core policy,
// session storage, signing keys, or private service implementations.
package customerauthcap

import (
	"context"
	"time"
)

// SessionConfirmation is a short-lived, exact-operation confirmation. It is
// not business authorization and never grants ownership or operation rights.
type SessionConfirmation struct {
	Required  bool
	Token     string
	ExpiresAt time.Time
}

// SessionConfirmationAccess is available only to the source resource owner.
// The digest must include its normalized server, action, parameters and request
// number. Core adds the trusted customer and validates the real host session.
type SessionConfirmationAccess interface {
	// PrepareCurrentConfirmation reads the authoritative merchant policy and
	// issues a single-use confirmation without a password or login-age gate.
	PrepareCurrentConfirmation(context.Context, string) (*SessionConfirmation, error)
	// ConsumeCurrentConfirmation must run in the shared transaction supplied by
	// WithCurrentEntitlement. It rechecks policy under a shared row lock and
	// consumes any required proof before the owner commits its operation task.
	// Missing, changed, expired and replayed proof or unavailable policy fail closed.
	ConsumeCurrentConfirmation(context.Context, string, string) error
}
