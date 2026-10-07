package versioncache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

func TestStandaloneInvalidationReloadsAuthority(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_42"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Basic"}`)})
	service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)

	first, err := service.Get(t.Context(), key)
	if err != nil || first.Source != SourceAuthority {
		t.Fatalf("first read=%+v err=%v", first, err)
	}
	first.Document[0] = 'x'
	second, err := service.Get(t.Context(), key)
	if err != nil || second.Source != SourceLocal || string(second.Document) != `{"name":"Basic"}` {
		t.Fatalf("local read=%+v err=%v", second, err)
	}
	authority.set(key, Snapshot{Version: "v2", Document: []byte(`{"name":"Plus"}`)})
	revision, err := service.Invalidate(t.Context(), key)
	if err != nil || revision == "" || revision == initialRevision {
		t.Fatalf("invalidate revision=%q err=%v", revision, err)
	}
	third, err := service.Get(t.Context(), key)
	if err != nil || third.Version != "v2" || string(third.Document) != `{"name":"Plus"}` {
		t.Fatalf("reloaded read=%+v err=%v", third, err)
	}
	loads, _ := authority.callCounts()
	if loads != 2 {
		t.Fatalf("authority loads=%d, want 2", loads)
	}
}

func TestConcurrentStandaloneMissRebuildsOnce(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := &blockingAuthority{
		snapshot: Snapshot{Version: "v1", Document: []byte(`{"name":"Concurrent"}`)},
		entered:  make(chan struct{}, 1), release: make(chan struct{}),
	}
	service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CONCURRENT"}
	const workers = 32
	start := make(chan struct{})
	errorsOut := make(chan error, workers)
	var ready sync.WaitGroup
	ready.Add(workers)
	for range workers {
		go func() {
			ready.Done()
			<-start
			result, err := service.Get(context.Background(), key)
			if err == nil && (result == nil || result.Version != "v1") {
				err = fmt.Errorf("unexpected result: %+v", result)
			}
			errorsOut <- err
		}()
	}
	ready.Wait()
	close(start)
	select {
	case <-authority.entered:
	case <-time.After(time.Second):
		t.Fatal("authority load did not start")
	}
	close(authority.release)
	for range workers {
		if err := <-errorsOut; err != nil {
			t.Fatalf("concurrent read: %v", err)
		}
	}
	if calls := authority.loads.Load(); calls != 1 {
		t.Fatalf("authority loads=%d, want 1", calls)
	}
}

func TestStandaloneGateWaitHonorsRequestCancellation(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := &blockingAuthority{
		snapshot: Snapshot{Version: "v1", Document: []byte(`{"name":"Blocking"}`)},
		entered:  make(chan struct{}, 1), release: make(chan struct{}),
	}
	service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)
	key := Key{Kind: KindPublishedProduct, ID: "PRD_CANCEL"}
	first := make(chan error, 1)
	go func() {
		_, err := service.Get(context.Background(), key)
		first <- err
	}()
	select {
	case <-authority.entered:
	case <-time.After(time.Second):
		t.Fatal("authority load did not start")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := service.Get(waitCtx, key); !bizerr.Is(err, CodeAuthorityUnavailable) {
		t.Fatalf("canceled gate error=%v, want authority unavailable", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("canceled gate waited %v", elapsed)
	}
	close(authority.release)
	if err := <-first; err != nil {
		t.Fatalf("first read: %v", err)
	}
}

func TestLocalRevisionCardinalityIsBoundedAndEvictionFailsSafe(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	service := newVersionCacheForTest(t, false, Config{MaximumEntries: 8}, authority, nil, &now)
	for index := range 64 {
		key := Key{Kind: KindPublishedProduct, ID: fmt.Sprintf("PRD_%d", index)}
		authority.set(key, Snapshot{
			Version: fmt.Sprintf("v%d", index), Document: []byte(fmt.Sprintf(`{"index":%d}`, index)),
		})
		if _, err := service.Invalidate(t.Context(), key); err != nil {
			t.Fatalf("invalidate %d: %v", index, err)
		}
	}
	service.local.mutex.Lock()
	revisions := len(service.local.revisions)
	service.local.mutex.Unlock()
	if revisions > 8 {
		t.Fatalf("local revision cardinality=%d, want <=8", revisions)
	}
	oldest := Key{Kind: KindPublishedProduct, ID: "PRD_0"}
	result, err := service.Get(t.Context(), oldest)
	if err != nil || result.Source != SourceAuthority || result.Version != "v0" {
		t.Fatalf("evicted revision read=%+v err=%v", result, err)
	}
}

func TestLocalStoreEnforcesAggregateByteBound(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	service := newVersionCacheForTest(t, false, Config{
		MaximumEntries: 10, MaximumLocalBytes: 320, MaximumSnapshotBytes: 256,
	}, authority, nil, &now)
	for index := range 4 {
		key := Key{Kind: KindPublishedProduct, ID: fmt.Sprintf("PRD_BYTES_%d", index)}
		authority.set(key, Snapshot{
			Version:  fmt.Sprintf("v%d", index),
			Document: []byte(fmt.Sprintf(`{"name":"%s"}`, strings.Repeat("x", 96))),
		})
		if _, err := service.Get(t.Context(), key); err != nil {
			t.Fatalf("prime %d: %v", index, err)
		}
		now = now.Add(time.Millisecond)
	}
	service.local.mutex.Lock()
	usedBytes, entries := service.local.usedBytes, len(service.local.entries)
	service.local.mutex.Unlock()
	if usedBytes > 320 || entries >= 4 {
		t.Fatalf("local store bytes=%d entries=%d", usedBytes, entries)
	}
}

func TestSnapshotValidationFailureIsAuthorityUnavailable(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	key := Key{Kind: KindPublishedProduct, ID: "PRD_INVALID_SNAPSHOT"}
	for name, snapshot := range map[string]Snapshot{
		"secret":       {Version: "v1", Document: []byte(`{"api_key":"hidden"}`)},
		"duplicate":    {Version: "v1", Document: []byte(`{"name":"a","name":"b"}`)},
		"bad version":  {Version: "v 1", Document: []byte(`{"name":"public"}`)},
		"invalid JSON": {Version: "v1", Document: []byte(`{"name":`)},
	} {
		t.Run(name, func(t *testing.T) {
			authority := newTestAuthority()
			authority.set(key, snapshot)
			service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)
			if _, err := service.Get(t.Context(), key); !bizerr.Is(err, CodeAuthorityUnavailable) {
				t.Fatalf("error=%v, want authority unavailable", err)
			}
		})
	}
}

func TestKeyAndConfigValidation(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)
	for _, key := range []Key{
		{Kind: Kind("funds"), ID: "balance"},
		{Kind: KindPublishedProduct, ID: ""},
		{Kind: KindPublishedProduct, ID: " leading"},
		{Kind: KindPublishedProduct, ID: "catalog/token/must-not-pass"},
		{Kind: KindPublishedProduct, ID: strings.Repeat("x", maximumKeyIDBytes+1)},
	} {
		if _, err := service.Get(t.Context(), key); !bizerr.Is(err, CodeInvalid) {
			t.Errorf("key=%+v error=%v, want invalid", key, err)
		}
	}
	for _, config := range []Config{
		{MaximumStale: -time.Second},
		{LockLease: time.Millisecond},
		{MaximumEntries: absoluteMaximumEntries + 1},
		{MaximumSnapshotBytes: 1024, MaximumLocalBytes: 512},
		{MaximumJSONNodes: absoluteMaximumJSONNodes + 1},
	} {
		if _, err := normalizeConfig(config); !bizerr.Is(err, CodeInvalid) {
			t.Errorf("config=%+v error=%v, want invalid", config, err)
		}
	}
}

func TestTypedNilAndClusterConstructionAreRejected(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_TYPED_NIL"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Context"}`)})
	service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)
	var typedNil *typedNilContext
	if _, err := service.Get(typedNil, key); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("typed-nil Get error=%v, want invalid", err)
	}
	if _, err := service.Invalidate(typedNil, key); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("typed-nil Invalidate error=%v, want invalid", err)
	}
	if _, err := newService(
		typedNil, Config{}, testMode{}, authority, nil, func() time.Time { return now },
	); err == nil {
		t.Fatal("typed-nil construction context was accepted")
	}
	if _, err := newService(
		t.Context(), Config{}, testMode{enabled: true}, authority, nil, func() time.Time { return now },
	); err == nil {
		t.Fatal("cluster construction without atomic backend was accepted")
	}
	if _, err := newService(
		t.Context(), Config{MaximumStale: time.Second}, testMode{enabled: true}, authority,
		newMemoryClusterBackend(), func() time.Time { return now },
	); err == nil {
		t.Fatal("cluster construction accepted clock skew at least as large as maximum stale")
	}
	if _, err := newService(
		t.Context(), Config{}, testMode{err: errors.New("config unavailable")}, authority,
		newMemoryClusterBackend(), func() time.Time { return now },
	); err == nil {
		t.Fatal("host mode read failure was accepted")
	}
}

func TestStandaloneExpiryDoesNotServeStaleData(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	key := Key{Kind: KindPublishedProduct, ID: "PRD_EXPIRED"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"name":"Public"}`)})
	service := newVersionCacheForTest(t, false, Config{MaximumStale: time.Second}, authority, nil, &now)
	if _, err := service.Get(t.Context(), key); err != nil {
		t.Fatalf("prime: %v", err)
	}
	now = now.Add(time.Second + time.Millisecond)
	authority.setLoadError(errors.New("database unavailable"))
	if _, err := service.Get(t.Context(), key); !bizerr.Is(err, CodeAuthorityUnavailable) {
		t.Fatalf("expired standalone error=%v, want authority unavailable", err)
	}
}

func TestStandaloneMetricsReflectHitsRebuildsAndInvalidations(t *testing.T) {
	now := time.Date(2026, time.August, 7, 1, 0, 0, 0, time.UTC)
	authority := newTestAuthority()
	key := Key{Kind: KindPublishedTemplate, ID: "welcome-en-US"}
	authority.set(key, Snapshot{Version: "v1", Document: []byte(`{"subject":"Welcome"}`)})
	service := newVersionCacheForTest(t, false, Config{}, authority, nil, &now)
	if _, err := service.Get(t.Context(), key); err != nil {
		t.Fatalf("authority read: %v", err)
	}
	if _, err := service.Get(t.Context(), key); err != nil {
		t.Fatalf("local read: %v", err)
	}
	if _, err := service.Invalidate(t.Context(), key); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	metrics := service.Metrics()
	if metrics.Rebuilds != 1 || metrics.LocalHits != 1 || metrics.Invalidations != 1 ||
		metrics.AuthorityLoads != 1 {
		t.Fatalf("metrics=%+v", metrics)
	}
}
