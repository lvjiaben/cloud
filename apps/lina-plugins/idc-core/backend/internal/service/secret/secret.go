// Package secret owns the only envelope-encrypted secret persistence and callback access implementation.
package secret

import (
	"context"
	"crypto/rand"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// Action is one exact operation checked by the injected scope authorizer.
type Action string

// Supported secret authorization actions.
const (
	ActionCreate        Action = "secret.create"
	ActionRecoverCreate Action = "secret.recover-create"
	ActionUpdate        Action = "secret.update"
	ActionMetadata      Action = "secret.metadata"
	ActionAccess        Action = "secret.access"
	ActionRevoke        Action = "secret.revoke"
	ActionRotate        Action = "secret.rotate"
)

// Scope is the public in-process scope contract reused by the owner
// implementation without exposing any persistence type.
type Scope = secretcap.Scope

// AuthorizationRequest carries only opaque or low-entropy metadata. Handle is
// redacted by its own formatting contract and is zero for scope-wide actions.
type AuthorizationRequest struct {
	Scope   Scope
	Action  Action
	Handle  secretcap.Handle
	Purpose secretcap.Purpose
}

// Authorizer validates that an authenticated caller may perform an exact
// action in an exact scope. Implementations must fail closed and whitelist
// access Purpose codes for each caller scope.
type Authorizer interface {
	Authorize(ctx context.Context, request AuthorizationRequest) error
}

// KeyMaterial is a detached deployment master key. Key must contain exactly
// 32 bytes. Providers must return a newly owned, destroyable byte slice on
// every call; the service clears that slice after copying it.
type KeyMaterial struct {
	ID  string
	Key []byte
}

// KeyProvider supplies deployment-owned master keys without persisting them.
// Active is the only key allowed for new encryption. Resolve may return older
// keys for decryption and accepts a batch to avoid per-wrapper remote lookups.
// Every returned key slice must be a detached, destroyable copy. A key ID is
// immutable: Active and Resolve must always return the same bytes for one ID,
// and an ID must never be reused for replacement key material.
type KeyProvider interface {
	Active(ctx context.Context) (KeyMaterial, error)
	Resolve(ctx context.Context, keyIDs []string) (map[string][]byte, error)
}

// Config defines bounded secret and rotation behavior. Zero values select
// safe defaults; negative values are rejected.
type Config struct {
	MaximumPlaintextSize int
	MaximumRotationBatch int
	MaximumKeyWraps      int
}

// Metadata is the only safe status projection. It never contains a handle,
// ciphertext, key identifier, version number, or plaintext-derived digest.
type Metadata struct {
	Configured bool
	MaskedTail string
	UpdatedAt  time.Time
}

// CreateInput creates one owner-scoped secret under an idempotency key.
type CreateInput struct {
	Scope          Scope
	IdempotencyKey string
	Plaintext      []byte
}

// CreateGeneratedInput creates one envelope-owned random credential without
// exposing its plaintext to the lifecycle caller.
type CreateGeneratedInput struct {
	Scope          Scope
	IdempotencyKey string
	Policy         string
}

// CreateResult returns the opaque owner handle separately from safe metadata.
type CreateResult struct {
	Handle   secretcap.Handle
	Metadata Metadata
	Reused   bool
}

// RecoverCreateInput locates only one fully committed create replay in the
// exact authorized scope. It never accepts or reconstructs plaintext.
type RecoverCreateInput struct {
	Scope          Scope
	IdempotencyKey string
}

// UpdateInput appends one immutable payload version using optimistic concurrency.
type UpdateInput struct {
	Scope             Scope
	Handle            secretcap.Handle
	ExpectedUpdatedAt time.Time
	IdempotencyKey    string
	Plaintext         []byte
}

// UpdateResult reports only safe metadata and idempotent reuse state.
type UpdateResult struct {
	Metadata Metadata
	Reused   bool
}

// RevokeInput permanently blocks callback access to one scoped handle.
type RevokeInput struct {
	Scope             Scope
	Handle            secretcap.Handle
	ExpectedUpdatedAt time.Time
}

// RotateInput rewraps a bounded scope batch with the active master key. It
// never returns or enumerates secret handles.
type RotateInput struct {
	Scope Scope
	Limit int
}

// RotateResult exposes aggregate progress without revealing secret identity.
type RotateResult struct {
	Rewrapped int
	Remaining bool
}

// Service defines the plugin-internal secret lifecycle boundary.
type Service interface {
	// Create persists one encrypted secret and returns its opaque handle once.
	Create(ctx context.Context, input CreateInput) (*CreateResult, error)
	// RecoverCreate returns the original handle only after the exact create
	// transaction has committed completely.
	RecoverCreate(ctx context.Context, input RecoverCreateInput) (*CreateResult, error)
	// CreateGenerated generates and persists one policy-bound random secret.
	CreateGenerated(ctx context.Context, input CreateGeneratedInput) (*CreateResult, error)
	// Update appends one encrypted payload version behind an optimistic fence.
	Update(ctx context.Context, input UpdateInput) (*UpdateResult, error)
	// Metadata returns the minimal safe projection for one already-held handle.
	Metadata(ctx context.Context, scope Scope, handle secretcap.Handle) (Metadata, error)
	// Revoke permanently prevents future callback access to one secret.
	Revoke(ctx context.Context, input RevokeInput) (Metadata, error)
	// Rotate additively rewraps a bounded scope batch with the active key.
	Rotate(ctx context.Context, input RotateInput) (RotateResult, error)
	// Accessor binds a caller scope to the callback-only plaintext boundary.
	Accessor(scope Scope) (secretcap.Accessor, error)
	// Manager binds the same scope to source-plugin lifecycle operations.
	Manager(scope Scope) (secretcap.Manager, error)
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

// serviceImpl owns envelope crypto and persistence through generated DAOs.
type serviceImpl struct {
	config     Config
	keys       KeyProvider
	authorizer Authorizer
	now        func() time.Time
	random     io.Reader
}

// New validates explicit dependencies and constructs the secret service. Key
// material must come from deployment configuration or an external KMS-backed
// provider; this package never reads a database key fallback.
func New(config Config, keys KeyProvider, authorizer Authorizer) (Service, error) {
	return newService(config, keys, authorizer, time.Now, rand.Reader)
}

// newService injects deterministic clock and randomness for focused tests.
func newService(
	config Config,
	keys KeyProvider,
	authorizer Authorizer,
	now func() time.Time,
	random io.Reader,
) (*serviceImpl, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	if nilDependency(keys) {
		return nil, gerror.New("secret service requires an explicit key provider")
	}
	if nilDependency(authorizer) {
		return nil, gerror.New("secret service requires an explicit scope authorizer")
	}
	if now == nil {
		return nil, gerror.New("secret service requires a clock")
	}
	if nilDependency(random) {
		return nil, gerror.New("secret service requires a cryptographic random source")
	}
	return &serviceImpl{
		config:     normalized,
		keys:       keys,
		authorizer: authorizer,
		now:        now,
		random:     random,
	}, nil
}

// nilDependency rejects typed-nil interface values before they can panic on
// their first security-sensitive use.
func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
