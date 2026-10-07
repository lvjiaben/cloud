// This file owns bounded in-process counters for cache observability.

package versioncache

import "sync/atomic"

// Metrics is a point-in-time copy safe for logging or a metrics adapter.
type Metrics struct {
	// LocalHits counts reads served from the current process.
	LocalHits uint64
	// SharedHits counts reads restored from the persistent cluster backend.
	SharedHits uint64
	// AuthorityLoads counts database-backed loader calls, including failures.
	AuthorityLoads uint64
	// WatermarkLoads counts durable authoritative revision checks.
	WatermarkLoads uint64
	// Rebuilds counts successful authoritative cache rebuilds.
	Rebuilds uint64
	// Invalidations counts successfully published revisions.
	Invalidations uint64
	// BackendFailures counts persistent state or lease coordination failures.
	BackendFailures uint64
	// DegradedReads counts successful reads completed without healthy coordination.
	DegradedReads uint64
	// StaleReads counts explicit bounded last-good responses.
	StaleReads uint64
	// Contentions counts rebuild attempts that did not acquire the fenced lease.
	Contentions uint64
	// CorruptEntries counts malformed persistent metadata or shared values.
	CorruptEntries uint64
	// InvalidationFailures counts revisions that could not be published.
	InvalidationFailures uint64
	// LockReleaseFailures counts releases left to lease expiry.
	LockReleaseFailures uint64
}

type metricCounters struct {
	localHits            atomic.Uint64
	sharedHits           atomic.Uint64
	authorityLoads       atomic.Uint64
	watermarkLoads       atomic.Uint64
	rebuilds             atomic.Uint64
	invalidations        atomic.Uint64
	backendFailures      atomic.Uint64
	degradedReads        atomic.Uint64
	staleReads           atomic.Uint64
	contentions          atomic.Uint64
	corruptEntries       atomic.Uint64
	invalidationFailures atomic.Uint64
	lockReleaseFailures  atomic.Uint64
}

// Metrics returns an immutable observation snapshot.
func (s *serviceImpl) Metrics() Metrics {
	return Metrics{
		LocalHits: s.metrics.localHits.Load(), SharedHits: s.metrics.sharedHits.Load(),
		AuthorityLoads: s.metrics.authorityLoads.Load(), Rebuilds: s.metrics.rebuilds.Load(),
		WatermarkLoads: s.metrics.watermarkLoads.Load(),
		Invalidations:  s.metrics.invalidations.Load(), BackendFailures: s.metrics.backendFailures.Load(),
		DegradedReads: s.metrics.degradedReads.Load(), StaleReads: s.metrics.staleReads.Load(),
		Contentions: s.metrics.contentions.Load(), CorruptEntries: s.metrics.corruptEntries.Load(),
		InvalidationFailures: s.metrics.invalidationFailures.Load(),
		LockReleaseFailures:  s.metrics.lockReleaseFailures.Load(),
	}
}
