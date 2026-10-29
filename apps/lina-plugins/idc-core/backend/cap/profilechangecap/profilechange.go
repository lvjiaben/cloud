// Package profilechangecap defines the verified self-profile write boundary.
// Notification providers validate short-lived proofs; only the existing host
// route owns the actual identity write. No host storage is exposed here.
package profilechangecap

import "context"

// ProviderPluginID owns login-contact verification and its durable evidence.
const ProviderPluginID = "idc-notify"

// Input binds the exact requested contact patch to both verification steps.
type Input struct {
	Email           *string
	Phone           *string
	OldVerification string
	NewVerification string
}

// Write executes the original authenticated route with the supplied transaction
// context. It must return the route error without flushing a success response.
type Write func(context.Context) error

// Provider validates and serializes identity contact writes using plugin state.
type Provider interface {
	// Apply verifies the current customer, session, old snapshot and new target,
	// consumes both proofs once, and runs write in the same database transaction.
	Apply(context.Context, Input, Write) error
	// Serialize runs an already-authorized administrative contact write under the
	// same account lock. It grants no authority; write retains host authorization.
	Serialize(context.Context, int, Write) error
}

// RecoveryProvider consumes one verified recovery code with the original password
// write in the same transaction. The proof is never a general profile grant.
type RecoveryProvider interface {
	ApplyRecovery(context.Context, string, Write) error
}
