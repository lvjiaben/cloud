// Package versioncache provides bounded, version-invalidated acceleration for
// public and rebuildable IDC snapshots. It is never a business authority.
package versioncache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/hostconfigcap"

	"lina-plugin-idc-core/backend/internal/service/versioncache/internal/coordination"
)

// Kind is the closed catalog of data allowed in this cache.
type Kind string

const (
	// KindPublishedProduct contains customer-visible, already-published catalog data.
	KindPublishedProduct Kind = "published_product"
	// KindPublishedTemplate contains published, non-secret presentation or message data.
	KindPublishedTemplate Kind = "published_template"
	// KindRebuildableAggregate contains a derived snapshot that can always be rebuilt.
	KindRebuildableAggregate Kind = "rebuildable_aggregate"
)

// Source identifies how a result was obtained.
type Source string

const (
	SourceLocal      Source = "local"
	SourceShared     Source = "shared"
	SourceAuthority  Source = "authority"
	SourceStaleLocal Source = "stale_local"
)

// Key identifies one cacheable snapshot. ID is an opaque public business key,
// never a database primary key, credential, payment, order, or service secret.
type Key struct {
	Kind Kind
	ID   string
}

// Snapshot is returned by the authoritative database-backed loader. Document
// must be public/redacted JSON; Version is an opaque low-entropy source version.
type Snapshot struct {
	Version  string
	Document json.RawMessage
}

// Result is an immutable caller-owned snapshot projection.
type Result struct {
	Snapshot
	Revision string
	LoadedAt time.Time
	Source   Source
	Stale    bool
	Degraded bool
}

// Authority is the only source of truth. Watermark must return a durable,
// low-entropy revision that changes in the same authoritative transaction as
// the represented data. Implementations must use bounded, tenant-scoped
// database queries and must not call external networks.
type Authority interface {
	// Watermark returns the current persistent revision without returning the
	// full snapshot. A clustered cache verifies it before every cache hit.
	Watermark(ctx context.Context, key Key) (string, error)
	// Load returns the complete public/redacted snapshot. Snapshot.Version must
	// equal the durable Watermark observed for the same committed state.
	Load(ctx context.Context, key Key) (Snapshot, error)
}

// ClusterBackend is the atomic persistent state and fenced-lease contract used
// in cluster mode. See the aliased interface for its transactional guarantees.
type ClusterBackend = coordination.Backend

// ClusterState is durable per-key generation/watermark metadata plus an
// optional rebuildable payload.
type ClusterState = coordination.State

// ClusterValue is one optional rebuildable shared payload.
type ClusterValue = coordination.Value

// ClusterLease identifies one finite, monotonically fenced rebuild owner.
type ClusterLease = coordination.Lease

// ClusterAcquireOutput reports acquired or contended lease state.
type ClusterAcquireOutput = coordination.AcquireOutput

// ClusterPublishInput carries atomic owner, generation, and watermark checks.
type ClusterPublishInput = coordination.PublishInput

// ClusterPublishOutput reports an atomic publish result and current state.
type ClusterPublishOutput = coordination.PublishOutput

// Config defines finite memory, payload, staleness, revision, and lock bounds.
// Zero values select safe defaults; negative values are rejected.
type Config struct {
	// MaximumStale is the hard lifetime of every local or shared snapshot.
	MaximumStale time.Duration
	// LockLease bounds one cross-node rebuild coordination attempt.
	LockLease time.Duration
	// AllowedClockSkew tolerates a small forward skew in shared loaded-at values.
	// In cluster mode it must remain smaller than MaximumStale because remote
	// timestamps are conservatively aged by this full allowance.
	AllowedClockSkew time.Duration
	// MaximumEntries bounds process-local key cardinality.
	MaximumEntries int
	// MaximumLocalBytes bounds aggregate process-local snapshot memory.
	MaximumLocalBytes int
	// MaximumSnapshotBytes bounds one canonical JSON document.
	MaximumSnapshotBytes int
	// MaximumJSONDepth bounds recursive validation work.
	MaximumJSONDepth int
	// MaximumJSONNodes bounds aggregate JSON validation work.
	MaximumJSONNodes int
}

// Service exposes reads, post-commit invalidation, and observability. Callers
// must invoke Invalidate only after the authoritative transaction commits, and
// must durably retry a failed invalidation through an outbox or managed job.
type Service interface {
	// Get returns a detached snapshot and explicit source/degradation evidence.
	Get(ctx context.Context, key Key) (*Result, error)
	// Invalidate publishes a new exact-key revision after authoritative commit.
	Invalidate(ctx context.Context, key Key) (string, error)
	// Metrics returns a point-in-time copy of cache counters.
	Metrics() Metrics
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

type serviceImpl struct {
	config         Config
	clusterEnabled bool
	authority      Authority
	coordination   *coordination.Controller
	now            func() time.Time
	local          localStore
	gates          keyedGates
	metrics        metricCounters
}

// New creates a version cache and reads the canonical cluster.enabled switch
// from the injected host configuration capability.
func New(
	ctx context.Context,
	config Config,
	mode hostconfigcap.Service,
	authority Authority,
	backend ClusterBackend,
) (Service, error) {
	return newService(ctx, config, mode, authority, backend, time.Now)
}

func newService(
	ctx context.Context,
	config Config,
	mode hostconfigcap.Service,
	authority Authority,
	backend ClusterBackend,
	now func() time.Time,
) (*serviceImpl, error) {
	if isNilInterface(ctx) || isNilInterface(mode) || isNilInterface(authority) ||
		isNilInterface(now) {
		return nil, gerror.New("version cache requires context, host mode, authority, and clock")
	}
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	clusterEnabled, err := mode.Bool(ctx, "cluster.enabled", false)
	if err != nil {
		return nil, gerror.Wrap(err, "read host cluster mode for version cache")
	}
	if clusterEnabled && normalized.AllowedClockSkew >= normalized.MaximumStale {
		return nil, gerror.New("clustered version cache requires clock skew below maximum stale")
	}
	var clusterCoordination *coordination.Controller
	if clusterEnabled {
		if isNilInterface(backend) {
			return nil, gerror.New("clustered version cache requires an atomic persistent cluster backend")
		}
		clusterCoordination, err = coordination.New(backend, normalized.LockLease)
		if err != nil {
			return nil, gerror.Wrap(err, "create version cache cluster coordination")
		}
	}
	service := &serviceImpl{
		config: normalized, clusterEnabled: clusterEnabled, authority: authority,
		coordination: clusterCoordination, now: now,
	}
	service.local.initialize(normalized.MaximumEntries, normalized.MaximumLocalBytes)
	service.gates.initialize()
	return service, nil
}
