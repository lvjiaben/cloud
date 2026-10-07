// Package coordination owns the atomic shared-state and fenced-lease contract
// used by the version cache. The backend must persist metadata and enforce the
// lease and state preconditions in the same atomic publish operation.
package coordination

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	// ErrLeaseLost reports that a caller no longer owns a live fenced lease.
	ErrLeaseLost = gerror.New("version cache fenced lease was lost")
	// ErrStateChanged reports that invalidation or another publisher advanced
	// the shared state before this caller could publish.
	ErrStateChanged = gerror.New("version cache shared state changed")
)

// Value is one rebuildable shared payload. Document must already be canonical,
// public/redacted JSON and Version must equal the enclosing State watermark.
type Value struct {
	Version  string
	Document []byte
	LoadedAt time.Time
}

// State is durable per-key coordination metadata. Generation and Watermark
// must survive value expiry, cache-process restart, and node restart. Value is
// optional acceleration data and may be cleared without removing metadata.
type State struct {
	Generation uint64
	Watermark  string
	Value      *Value
}

// Lease identifies one monotonically fenced, finite rebuild ownership grant.
type Lease struct {
	Key      string
	Owner    string
	Fence    uint64
	ExpireAt time.Time
}

// AcquireOutput reports whether this caller acquired the rebuild lease. A
// contended response must still include the current finite ExpireAt value.
type AcquireOutput struct {
	Acquired bool
	Lease    Lease
}

// PublishInput carries every condition needed for one atomic shared publish.
type PublishInput struct {
	Key                 string
	Lease               Lease
	ExpectedGeneration  uint64
	ExpectedWatermark   string
	AuthoritativeMarker string
	Value               Value
}

// PublishOutput reports whether the conditional publish committed and returns
// the current durable state in either case.
type PublishOutput struct {
	Published bool
	State     State
}

// Backend is the version cache's cluster-owned coordination seam. Implementors
// must use one shared persistent backend for all nodes and obey these rules:
//
//   - Advance atomically increments Generation, stores watermark, and clears
//     Value without ever deleting the durable generation/watermark metadata.
//   - Acquire returns a monotonically increasing Fence and a finite ExpireAt.
//     Renew succeeds only for the same live owner/fence and returns its new
//     ExpireAt. Every method must promptly honor context cancellation.
//   - Publish atomically verifies the lease owner, fence, and backend expiry,
//     verifies ExpectedGeneration/ExpectedWatermark against current State, and
//     only then stores AuthoritativeMarker and Value. A pre-write or post-write
//     lease check outside this method is not an acceptable substitute.
//   - Release is idempotent and must never release a newer owner's lease.
//
// A host cache Set combined with an unrelated distributed lock cannot satisfy
// Publish and therefore must not be adapted to this interface without an
// atomic transaction, Lua script, compare-and-swap, or equivalent primitive.
type Backend interface {
	// Read returns the durable state and whether the key has ever been initialized.
	Read(ctx context.Context, key string) (State, bool, error)
	// Advance atomically increments generation, records watermark, and clears Value.
	Advance(ctx context.Context, key string, watermark string) (State, error)
	// Acquire obtains a new fenced lease or reports the current live lease.
	Acquire(ctx context.Context, key string, lease time.Duration) (*AcquireOutput, error)
	// Renew extends only the same currently live owner/fence lease.
	Renew(ctx context.Context, lease Lease) (Lease, error)
	// Publish performs all lease and expected-state checks with the write atomically.
	Publish(ctx context.Context, input PublishInput) (*PublishOutput, error)
	// Release idempotently releases only the matching owner/fence lease.
	Release(ctx context.Context, lease Lease) error
}

// CloneState returns detached slices so local callers cannot mutate backend
// state through a returned document.
func CloneState(state State) State {
	if state.Value == nil {
		return state
	}
	value := *state.Value
	value.Document = append([]byte(nil), value.Document...)
	state.Value = &value
	return state
}
