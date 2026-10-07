// This file defines the source-plugin-only accessor construction boundary.

package secretcap

import (
	"context"
	"time"
)

const (
	// MaximumManagedPlaintextBytes is the hard source-manager input bound.
	MaximumManagedPlaintextBytes = 1 << 20
	// GeneratedSecretPolicyURLSafe256 creates 256 random bits encoded without
	// padding in the URL-safe Base64 alphabet.
	GeneratedSecretPolicyURLSafe256 = "url_safe_256"
)

// Metadata is the only safe secret status projection. It never contains a
// handle, ciphertext, key identifier, version, or plaintext-derived digest.
type Metadata struct {
	// Configured reports whether the exact handle currently resolves to an active secret.
	Configured bool
	// MaskedTail is a bounded display-only suffix and is never accepted as identity.
	MaskedTail string
	// UpdatedAt is the optimistic-concurrency fence for lifecycle mutations.
	UpdatedAt time.Time
}

// PlaintextProducer supplies one complete plaintext view synchronously through
// consume. The producer must invoke consume exactly once and must not retain,
// log, serialize, or reuse the view after consume returns.
type PlaintextProducer func(consume Consumer) error

// CreateResult is an in-process result. Handle deliberately rejects JSON and
// text serialization; only an authorized owner may persist OpaqueRef.
type CreateResult struct {
	// Handle is the new or idempotently reused opaque in-process handle.
	Handle Handle
	// Metadata is the safe status projection for the handle.
	Metadata Metadata
	// Reused reports whether the idempotency key replayed an earlier create.
	Reused bool
}

// MutationResult reports safe metadata and idempotent replay state.
type MutationResult struct {
	// Metadata is the safe post-mutation status projection.
	Metadata Metadata
	// Reused reports whether the idempotency key replayed an earlier mutation.
	Reused bool
}

// RotationResult reports bounded progress without enumerating handles.
type RotationResult struct {
	// Rewrapped is the bounded number of envelopes rewrapped in this call.
	Rewrapped int
	// Remaining reports whether another bounded rotation call is required.
	Remaining bool
}

// Manager is the source-plugin-only lifecycle boundary bound to one exact
// scope. It accepts plaintext only through a synchronous producer and never
// offers listing or handle discovery.
type Manager interface {
	// Create stores one producer-supplied value inside the manager's exact scope.
	Create(ctx context.Context, idempotencyKey string, produce PlaintextProducer) (CreateResult, error)
	// RecoverCreate returns the already committed result for the exact scope and
	// create idempotency key. It accepts no plaintext and never discovers or
	// enumerates any other handle.
	RecoverCreate(ctx context.Context, idempotencyKey string) (CreateResult, error)
	// Replace mutates only the exact handle in scope and requires an optimistic fence.
	Replace(
		ctx context.Context,
		handle Handle,
		expectedUpdatedAt time.Time,
		idempotencyKey string,
		produce PlaintextProducer,
	) (MutationResult, error)
	// Metadata returns only a safe projection for one exact in-scope handle.
	Metadata(ctx context.Context, handle Handle) (Metadata, error)
	// Revoke disables one exact in-scope handle behind an optimistic fence.
	Revoke(ctx context.Context, handle Handle, expectedUpdatedAt time.Time) (Metadata, error)
	// Rotate rewraps at most limit envelopes in the exact scope without listing handles.
	Rotate(ctx context.Context, limit int) (RotationResult, error)
}

// GeneratedManager is the optional owner-only extension for credentials that
// must be generated once inside the encrypted envelope boundary. Replaying an
// identical idempotency key and policy returns the original opaque handle and
// never produces a second plaintext value.
type GeneratedManager interface {
	Manager
	CreateGenerated(ctx context.Context, idempotencyKey string, policy string) (CreateResult, error)
}

// Source creates callback-only accessors during explicit source-plugin
// composition. It exposes no discovery, handle lookup, plaintext return,
// persistence implementation, or dynamic-plugin bridge.
type Source interface {
	// Accessor validates and binds one exact trusted scope. Authorization is
	// rechecked for every Access call made through the returned accessor.
	Accessor(scope Scope) (Accessor, error)
	// Manager validates and binds the same exact scope to lifecycle operations.
	// HTTP authorization, recent reauthentication, and audit remain the caller
	// owner's responsibility before invoking this in-process boundary.
	Manager(scope Scope) (Manager, error)
}
