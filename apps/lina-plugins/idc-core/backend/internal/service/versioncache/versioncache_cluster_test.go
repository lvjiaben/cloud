package versioncache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

func TestClusterInvalidationIsVisibleAcrossNodes(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedTemplate, ID: "welcome-en-US"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"subject":"Welcome"}`)})
	nodeA := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	nodeB := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)

	if result, err := nodeA.Get(t.Context(), key); err != nil || result.Source != SourceAuthority {
		t.Fatalf("node A initial read=%+v err=%v", result, err)
	}
	if result, err := nodeB.Get(t.Context(), key); err != nil || result.Source != SourceShared {
		t.Fatalf("node B shared read=%+v err=%v", result, err)
	}
	authority.set(key, Snapshot{Version: "v2", Document: []byte(`{"subject":"Hello"}`)})
	revision, err := nodeA.Invalidate(t.Context(), key)
	if err != nil || revision == "" {
		t.Fatalf("cluster invalidate revision=%q err=%v", revision, err)
	}
	result, err := nodeB.Get(t.Context(), key)
	if err != nil || result.Version != "v2" || result.Source != SourceAuthority {
		t.Fatalf("node B post-invalidation read=%+v err=%v", result, err)
	}
	loads, _ := authority.callCounts()
	if loads != 2 {
		t.Fatalf("authority loads=%d, want 2", loads)
	}
}

func TestClusterBackendFailureUsesAuthorityThenBoundedStale(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindRebuildableAggregate, ID: "public-catalog"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"count":2}`)})
	service := newVersionCacheForTest(t, true, Config{MaximumStale: time.Minute}, authority, backend, &now)
	first, err := service.Get(t.Context(), key)
	if err != nil || first.Source != SourceAuthority || first.Degraded || first.Stale {
		t.Fatalf("authority result=%+v err=%v", first, err)
	}
	now = now.Add(30 * time.Second)
	backend.setFailures(true, false, false)
	authority.setLoadError(errors.New("database unavailable"))
	stale, err := service.Get(t.Context(), key)
	if err != nil || !stale.Stale || !stale.Degraded || stale.Source != SourceStaleLocal {
		t.Fatalf("bounded stale result=%+v err=%v", stale, err)
	}
	now = now.Add(31 * time.Second)
	if _, err = service.Get(t.Context(), key); !bizerr.Is(err, CodeAuthorityUnavailable) {
		t.Fatalf("expired stale error=%v, want authority unavailable", err)
	}
	metrics := service.Metrics()
	if metrics.StaleReads != 1 || metrics.DegradedReads != 1 || metrics.BackendFailures == 0 {
		t.Fatalf("degradation metrics=%+v", metrics)
	}
}

func TestClusterBackendFailureFreshNodeLoadsAuthorityInDegradedMode(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_FRESH_DEGRADED"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Direct"}`)})
	backend.setFailures(true, false, false)
	service := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	result, err := service.Get(t.Context(), key)
	if err != nil || result.Source != SourceAuthority || !result.Degraded || result.Stale {
		t.Fatalf("fresh degraded result=%+v err=%v", result, err)
	}
}

func TestClusterLeaseFailureFreshNodeLoadsAuthorityInDegradedMode(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_LEASE_DEGRADED"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Direct"}`)})
	backend.setFailures(false, false, true)
	service := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	result, err := service.Get(t.Context(), key)
	if err != nil || result.Source != SourceAuthority || !result.Degraded || result.Stale {
		t.Fatalf("lease-failure degraded result=%+v err=%v", result, err)
	}
}

func TestPersistentWatermarkPreventsRegressionAfterBackendRecoveryAndRestart(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_RECOVERY"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Old"}`)})
	nodeA := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	if _, err := nodeA.Get(t.Context(), key); err != nil {
		t.Fatalf("prime old shared value: %v", err)
	}

	// Simulate a committed authoritative write whose best-effort cluster
	// invalidation was missed while the backend was unavailable.
	authority.set(key, Snapshot{Version: "v2", Document: []byte(`{"name":"Current"}`)})
	backend.setFailures(true, false, false)
	degradedA, err := nodeA.Get(t.Context(), key)
	if err != nil || degradedA.Version != "v2" || !degradedA.Degraded {
		t.Fatalf("node A degraded read=%+v err=%v", degradedA, err)
	}
	nodeB := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	degradedB, err := nodeB.Get(t.Context(), key)
	if err != nil || degradedB.Version != "v2" || !degradedB.Degraded {
		t.Fatalf("restarted node degraded read=%+v err=%v", degradedB, err)
	}

	backend.setFailures(false, false, false)
	nodeAfterRestart := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	recovered, err := nodeAfterRestart.Get(t.Context(), key)
	if err != nil || recovered.Version != "v2" || recovered.Source != SourceAuthority || recovered.Degraded {
		t.Fatalf("recovered node regressed to pre-fault shared state: result=%+v err=%v", recovered, err)
	}
	fromOtherProcess, err := nodeA.Get(t.Context(), key)
	if err != nil || fromOtherProcess.Version != "v2" {
		t.Fatalf("existing node regressed after recovery: result=%+v err=%v", fromOtherProcess, err)
	}
	state, found := backend.state(stableStorageKey(key))
	if !found || state.Watermark != "v2" || state.Value == nil || state.Value.Version != "v2" {
		t.Fatalf("recovered persistent state=%+v found=%v", state, found)
	}
}

func TestFailedInvalidationCannotExposePreCommitSharedValueToFreshNode(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_FAILED_INVALIDATE"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Old"}`)})
	nodeA := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	if _, err := nodeA.Get(t.Context(), key); err != nil {
		t.Fatalf("prime: %v", err)
	}
	authority.set(key, Snapshot{Version: "v2", Document: []byte(`{"name":"New"}`)})
	backend.setFailures(false, true, false)
	if _, err := nodeA.Invalidate(t.Context(), key); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("failed invalidation error=%v, want unavailable", err)
	}
	backend.setFailures(false, false, false)
	freshNode := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	result, err := freshNode.Get(t.Context(), key)
	if err != nil || result.Version != "v2" || string(result.Document) != `{"name":"New"}` {
		t.Fatalf("fresh-node recovery result=%+v err=%v", result, err)
	}
}

func TestRecoveredBackendCoalescesThirtyTwoConcurrentReloads(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_RECOVERY_COALESCED"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Old"}`)})
	service := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	if _, err := service.Get(t.Context(), key); err != nil {
		t.Fatalf("prime: %v", err)
	}
	authority.set(key, Snapshot{Version: "v2", Document: []byte(`{"name":"Degraded"}`)})
	backend.setFailures(true, false, false)
	if result, err := service.Get(t.Context(), key); err != nil || !result.Degraded {
		t.Fatalf("degraded read=%+v err=%v", result, err)
	}
	authority.set(key, Snapshot{Version: "v3", Document: []byte(`{"name":"Recovered"}`)})
	backend.setFailures(false, false, false)

	const workers = 32
	start := make(chan struct{})
	outcomes := make(chan error, workers)
	var ready sync.WaitGroup
	ready.Add(workers)
	for range workers {
		go func() {
			ready.Done()
			<-start
			result, err := service.Get(context.Background(), key)
			if err == nil && (result == nil || result.Version != "v3") {
				err = fmt.Errorf("unexpected recovery result: %+v", result)
			}
			outcomes <- err
		}()
	}
	ready.Wait()
	close(start)
	for range workers {
		if err := <-outcomes; err != nil {
			t.Fatalf("concurrent recovery: %v", err)
		}
	}
	loads, _ := authority.callCounts()
	if loads != 3 {
		t.Fatalf("authority loads=%d, want prime + degraded + one recovered", loads)
	}
}

func TestThirtyTwoNodesWaitForOneClusterWinner(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := &blockingAuthority{
		snapshot: Snapshot{Version: "v1", Document: []byte(`{"name":"Winner"}`)},
		entered:  make(chan struct{}, 1), release: make(chan struct{}),
	}
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CLUSTER_32"}
	const workers = 32
	nodes := make([]*serviceImpl, workers)
	for index := range nodes {
		nodes[index] = newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	}
	start := make(chan struct{})
	type outcome struct {
		result *Result
		err    error
	}
	outcomes := make(chan outcome, workers)
	var ready sync.WaitGroup
	ready.Add(workers)
	for index := range workers {
		go func(node *serviceImpl) {
			ready.Done()
			<-start
			result, err := node.Get(context.Background(), key)
			outcomes <- outcome{result: result, err: err}
		}(nodes[index])
	}
	ready.Wait()
	close(start)
	select {
	case <-authority.entered:
	case <-time.After(time.Second):
		t.Fatal("winning authority load did not start")
	}
	time.Sleep(40 * time.Millisecond)
	close(authority.release)
	authoritySources := 0
	for range workers {
		got := <-outcomes
		if got.err != nil || got.result == nil || got.result.Version != "v1" {
			t.Fatalf("cluster outcome=%+v err=%v", got.result, got.err)
		}
		if got.result.Source == SourceAuthority {
			authoritySources++
		}
	}
	if calls := authority.loads.Load(); calls != 1 || authoritySources != 1 {
		t.Fatalf("authority loads=%d authority results=%d, want 1/1", calls, authoritySources)
	}
}

func TestClusterContentionCancellationNeverBypassesHeldLease(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := &blockingAuthority{
		snapshot: Snapshot{Version: "v1", Document: []byte(`{"name":"Held"}`)},
		entered:  make(chan struct{}, 1), release: make(chan struct{}),
	}
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CONTENTION_CANCEL"}
	winner := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	waiter := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	first := make(chan error, 1)
	go func() {
		_, err := winner.Get(context.Background(), key)
		first <- err
	}()
	select {
	case <-authority.entered:
	case <-time.After(time.Second):
		t.Fatal("winner did not enter authority")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if _, err := waiter.Get(waitCtx, key); !bizerr.Is(err, CodeAuthorityUnavailable) {
		t.Fatalf("contended cancellation error=%v, want authority unavailable", err)
	}
	if calls := authority.loads.Load(); calls != 1 {
		t.Fatalf("contended waiter bypassed lease; authority loads=%d", calls)
	}
	close(authority.release)
	if err := <-first; err != nil {
		t.Fatalf("winner read: %v", err)
	}
}

func TestClusterReleaseUsesCancellationIndependentContext(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	key := Key{Kind: KindPublishedProduct, ID: "PRD_RELEASE_CANCEL"}
	requestCtx, cancel := context.WithCancel(context.Background())
	authority := &cancelAfterLoadAuthority{
		cancel: cancel, snapshot: Snapshot{Version: "v1", Document: []byte(`{"name":"Cancel"}`)},
	}
	backend := newMemoryClusterBackend()
	service := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	if _, err := service.Get(requestCtx, key); !bizerr.Is(err, CodeAuthorityUnavailable) {
		t.Fatalf("canceled rebuild error=%v, want authority unavailable", err)
	}
	_, _, _, _, _, releases := backend.counts()
	if releases != 1 {
		t.Fatalf("release count=%d, want 1", releases)
	}
	if backend.hasLease(stableStorageKey(key)) {
		t.Fatal("lease remained held after canceled request")
	}
}

func TestSlowClusterRebuildRenewsLease(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := &blockingAuthority{
		snapshot: Snapshot{Version: "v1", Document: []byte(`{"name":"Slow"}`)},
		entered:  make(chan struct{}, 1), release: make(chan struct{}),
	}
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_SLOW_RENEW"}
	nodeA := newVersionCacheForTest(t, true, Config{LockLease: time.Second}, authority, backend, &now)
	nodeB := newVersionCacheForTest(t, true, Config{LockLease: time.Second}, authority, backend, &now)
	type outcome struct {
		result *Result
		err    error
	}
	first := make(chan outcome, 1)
	go func() {
		result, err := nodeA.Get(context.Background(), key)
		first <- outcome{result: result, err: err}
	}()
	select {
	case <-authority.entered:
	case <-time.After(time.Second):
		t.Fatal("slow rebuild did not enter authority")
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for {
		_, _, _, renews, _, _ := backend.counts()
		if renews >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lease was not renewed three times; renews=%d", renews)
		}
		time.Sleep(10 * time.Millisecond)
	}
	second := make(chan outcome, 1)
	go func() {
		result, err := nodeB.Get(context.Background(), key)
		second <- outcome{result: result, err: err}
	}()
	time.Sleep(30 * time.Millisecond)
	if calls := authority.loads.Load(); calls != 1 {
		t.Fatalf("second node loaded authority after original lease duration; calls=%d", calls)
	}
	close(authority.release)
	if got := <-first; got.err != nil || got.result == nil || got.result.Source != SourceAuthority {
		t.Fatalf("first result=%+v err=%v", got.result, got.err)
	}
	if got := <-second; got.err != nil || got.result == nil || got.result.Source != SourceShared {
		t.Fatalf("second result=%+v err=%v", got.result, got.err)
	}
}

func TestSharedForwardSkewCannotExtendMaximumStale(t *testing.T) {
	base := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	now := base
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CLOCK_SKEW"}
	storageKey := stableStorageKey(key)
	backend := newMemoryClusterBackend()
	backend.setState(storageKey, ClusterState{
		Generation: 1, Watermark: "v1",
		Value: &ClusterValue{
			Version: "v1", Document: []byte(`{"name":"Skewed"}`), LoadedAt: base.Add(30 * time.Second),
		},
	})
	authority := newTestAuthority()
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Skewed"}`)})
	service := newVersionCacheForTest(t, true, Config{
		MaximumStale: time.Minute, AllowedClockSkew: 30 * time.Second,
	}, authority, backend, &now)
	first, err := service.Get(t.Context(), key)
	if err != nil || first.Source != SourceShared || !first.LoadedAt.Equal(base) {
		t.Fatalf("first skew read=%+v err=%v", first, err)
	}
	now = base.Add(61 * time.Second)
	authority.setLoadError(errors.New("authority unavailable"))
	if _, err = service.Get(t.Context(), key); !bizerr.Is(err, CodeAuthorityUnavailable) {
		t.Fatalf("expired skew read error=%v, want authority unavailable", err)
	}
}

func TestCorruptPersistentMetadataIsRepaired(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CORRUPT_META"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Repaired"}`)})
	backend.setState(stableStorageKey(key), ClusterState{Generation: 0, Watermark: "bad value"})
	service := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	result, err := service.Get(t.Context(), key)
	if err != nil || result.Version != "v1" {
		t.Fatalf("repair result=%+v err=%v", result, err)
	}
	state, found := backend.state(stableStorageKey(key))
	if !found || state.Generation == 0 || state.Watermark != "v1" || state.Value == nil {
		t.Fatalf("repaired state=%+v found=%v", state, found)
	}
	if metrics := service.Metrics(); metrics.CorruptEntries == 0 {
		t.Fatalf("corruption metric=%+v", metrics)
	}
}

func TestCorruptSharedPayloadIsNeverReturned(t *testing.T) {
	now := time.Date(2026, time.August, 7, 2, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	backend := newMemoryClusterBackend()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CORRUPT_VALUE"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Clean"}`)})
	backend.setState(stableStorageKey(key), ClusterState{
		Generation: 1, Watermark: "v1",
		Value: &ClusterValue{
			Version: "v1", Document: []byte(`{"token":"hidden"}`), LoadedAt: now,
		},
	})
	service := newVersionCacheForTest(t, true, Config{}, authority, backend, &now)
	result, err := service.Get(t.Context(), key)
	if err != nil || result.Version != "v1" || string(result.Document) != `{"name":"Clean"}` ||
		result.Source != SourceAuthority {
		t.Fatalf("corrupt-shared rebuild=%+v err=%v", result, err)
	}
}

type cancelAfterLoadAuthority struct {
	cancel   context.CancelFunc
	snapshot Snapshot
}

func (a *cancelAfterLoadAuthority) Watermark(ctx context.Context, _ Key) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return a.snapshot.Version, nil
}

func (a *cancelAfterLoadAuthority) Load(context.Context, Key) (Snapshot, error) {
	a.cancel()
	return a.snapshot, nil
}
