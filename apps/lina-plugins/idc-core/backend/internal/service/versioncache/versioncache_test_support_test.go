package versioncache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/container/gvar"

	"lina-core/pkg/plugin/capability/hostconfigcap"
)

func newVersionCacheForTest(
	t *testing.T,
	cluster bool,
	config Config,
	authority Authority,
	backend ClusterBackend,
	now *time.Time,
) *serviceImpl {
	t.Helper()
	if cluster && config.LockLease == 0 {
		config.LockLease = time.Second
	}
	service, err := newService(
		t.Context(), config, testMode{enabled: cluster}, authority, backend,
		func() time.Time { return *now },
	)
	if err != nil {
		t.Fatalf("create version cache: %v", err)
	}
	return service
}

type testMode struct {
	enabled bool
	err     error
}

func (m testMode) Bool(_ context.Context, key string, defaultValue bool) (bool, error) {
	if key != "cluster.enabled" || defaultValue {
		return false, fmt.Errorf("unexpected host mode read key=%q default=%v", key, defaultValue)
	}
	return m.enabled, m.err
}

func (testMode) Get(context.Context, string, any) (*gvar.Var, error) {
	return nil, errors.New("unexpected host config Get call")
}

func (testMode) Exists(context.Context, string) (bool, error) {
	return false, errors.New("unexpected host config Exists call")
}

func (testMode) String(context.Context, string, string) (string, error) {
	return "", errors.New("unexpected host config String call")
}

func (testMode) Int(context.Context, string, int) (int, error) {
	return 0, errors.New("unexpected host config Int call")
}

func (testMode) Duration(context.Context, string, time.Duration) (time.Duration, error) {
	return 0, errors.New("unexpected host config Duration call")
}

func (testMode) SysConfig() hostconfigcap.SysConfigService { return nil }

type testAuthority struct {
	mutex          sync.Mutex
	snapshots      map[string]Snapshot
	loadErr        error
	watermarkErr   error
	loads          int
	watermarkLoads int
}

func newTestAuthority() *testAuthority {
	return &testAuthority{snapshots: make(map[string]Snapshot)}
}

func (a *testAuthority) Watermark(ctx context.Context, key Key) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.watermarkLoads++
	if a.watermarkErr != nil {
		return "", a.watermarkErr
	}
	return a.snapshots[testAuthorityKey(key)].Version, nil
}

func (a *testAuthority) Load(ctx context.Context, key Key) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.loads++
	if a.loadErr != nil {
		return Snapshot{}, a.loadErr
	}
	snapshot := a.snapshots[testAuthorityKey(key)]
	snapshot.Document = append([]byte(nil), snapshot.Document...)
	return snapshot, nil
}

func (a *testAuthority) set(key Key, snapshot Snapshot) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	snapshot.Document = append([]byte(nil), snapshot.Document...)
	a.snapshots[testAuthorityKey(key)] = snapshot
	a.loadErr = nil
	a.watermarkErr = nil
}

func (a *testAuthority) setLoadError(err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.loadErr = err
}

func (a *testAuthority) setWatermarkError(err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.watermarkErr = err
}

func (a *testAuthority) callCounts() (loads int, watermarks int) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.loads, a.watermarkLoads
}

func testAuthorityKey(key Key) string {
	return string(key.Kind) + "\x00" + key.ID
}

type blockingAuthority struct {
	snapshot Snapshot
	entered  chan struct{}
	release  chan struct{}
	loads    atomic.Int32
}

func (a *blockingAuthority) Watermark(ctx context.Context, _ Key) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return a.snapshot.Version, nil
}

func (a *blockingAuthority) Load(ctx context.Context, _ Key) (Snapshot, error) {
	a.loads.Add(1)
	select {
	case a.entered <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	case <-a.release:
		snapshot := a.snapshot
		snapshot.Document = append([]byte(nil), snapshot.Document...)
		return snapshot, nil
	}
}

type typedNilContext struct{}

func (*typedNilContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*typedNilContext) Done() <-chan struct{}       { return nil }
func (*typedNilContext) Err() error                  { return nil }
func (*typedNilContext) Value(any) any               { return nil }

type memoryClusterBackend struct {
	mutex      sync.Mutex
	states     map[string]ClusterState
	leases     map[string]ClusterLease
	leaseTimes map[string]time.Duration
	nextFence  uint64
	reads      int
	advances   int
	acquires   int
	renews     int
	publishes  int
	releases   int
	failRead   bool
	failWrite  bool
	failLease  bool
	publishIn  chan struct{}
	publishOut chan struct{}
	renewIn    chan struct{}
	renewOut   chan struct{}
}

func newMemoryClusterBackend() *memoryClusterBackend {
	return &memoryClusterBackend{
		states: make(map[string]ClusterState), leases: make(map[string]ClusterLease),
		leaseTimes: make(map[string]time.Duration),
	}
}

func (b *memoryClusterBackend) Read(ctx context.Context, key string) (ClusterState, bool, error) {
	if err := ctx.Err(); err != nil {
		return ClusterState{}, false, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.reads++
	if b.failRead {
		return ClusterState{}, false, errors.New("cluster read unavailable")
	}
	state, found := b.states[key]
	return cloneClusterState(state), found, nil
}

func (b *memoryClusterBackend) Advance(
	ctx context.Context,
	key string,
	watermark string,
) (ClusterState, error) {
	if err := ctx.Err(); err != nil {
		return ClusterState{}, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.advances++
	if b.failWrite {
		return ClusterState{}, errors.New("cluster advance unavailable")
	}
	state := b.states[key]
	state.Generation++
	if state.Generation == 0 {
		return ClusterState{}, errors.New("cluster generation overflow")
	}
	state.Watermark = watermark
	state.Value = nil
	b.states[key] = cloneClusterState(state)
	return cloneClusterState(state), nil
}

func (b *memoryClusterBackend) Acquire(
	ctx context.Context,
	key string,
	leaseDuration time.Duration,
) (*ClusterAcquireOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.acquires++
	if b.failLease {
		return nil, errors.New("cluster lease unavailable")
	}
	now := time.Now()
	if held, found := b.leases[key]; found && held.ExpireAt.After(now) {
		return &ClusterAcquireOutput{Acquired: false, Lease: held}, nil
	}
	b.nextFence++
	lease := ClusterLease{
		Key: key, Owner: fmt.Sprintf("owner-%d", b.nextFence), Fence: b.nextFence,
		ExpireAt: now.Add(leaseDuration),
	}
	b.leases[key] = lease
	b.leaseTimes[key] = leaseDuration
	return &ClusterAcquireOutput{Acquired: true, Lease: lease}, nil
}

func (b *memoryClusterBackend) Renew(
	ctx context.Context,
	lease ClusterLease,
) (ClusterLease, error) {
	b.mutex.Lock()
	renewIn, renewOut := b.renewIn, b.renewOut
	b.mutex.Unlock()
	if renewIn != nil {
		select {
		case renewIn <- struct{}{}:
		default:
		}
	}
	if renewOut != nil {
		select {
		case <-ctx.Done():
			return ClusterLease{}, ctx.Err()
		case <-renewOut:
		}
	}
	if err := ctx.Err(); err != nil {
		return ClusterLease{}, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.renews++
	if b.failLease {
		return ClusterLease{}, errors.New("cluster renew unavailable")
	}
	held, found := b.leases[lease.Key]
	now := time.Now()
	if !found || !sameTestLease(held, lease) || !held.ExpireAt.After(now) {
		return ClusterLease{}, errors.New("cluster lease lost")
	}
	duration := b.leaseTimes[lease.Key]
	if duration <= 0 {
		return ClusterLease{}, errors.New("cluster lease duration missing")
	}
	held.ExpireAt = now.Add(duration)
	b.leases[lease.Key] = held
	return held, nil
}

func (b *memoryClusterBackend) Publish(
	ctx context.Context,
	input ClusterPublishInput,
) (*ClusterPublishOutput, error) {
	b.mutex.Lock()
	publishIn, publishOut := b.publishIn, b.publishOut
	b.mutex.Unlock()
	if publishIn != nil {
		select {
		case publishIn <- struct{}{}:
		default:
		}
	}
	if publishOut != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-publishOut:
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.publishes++
	if b.failWrite {
		return nil, errors.New("cluster publish unavailable")
	}
	held, found := b.leases[input.Key]
	if !found || !sameTestLease(held, input.Lease) || !held.ExpireAt.After(time.Now()) {
		return &ClusterPublishOutput{State: cloneClusterState(b.states[input.Key])}, nil
	}
	state := b.states[input.Key]
	if state.Generation != input.ExpectedGeneration || state.Watermark != input.ExpectedWatermark {
		return &ClusterPublishOutput{State: cloneClusterState(state)}, nil
	}
	if state.Generation == 0 || state.Watermark != input.AuthoritativeMarker {
		state.Generation++
		if state.Generation == 0 {
			return nil, errors.New("cluster generation overflow")
		}
	}
	value := input.Value
	value.Document = append([]byte(nil), value.Document...)
	state.Watermark = input.AuthoritativeMarker
	state.Value = &value
	b.states[input.Key] = cloneClusterState(state)
	return &ClusterPublishOutput{Published: true, State: cloneClusterState(state)}, nil
}

func (b *memoryClusterBackend) Release(ctx context.Context, lease ClusterLease) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.releases++
	if held, found := b.leases[lease.Key]; found && sameTestLease(held, lease) {
		delete(b.leases, lease.Key)
		delete(b.leaseTimes, lease.Key)
	}
	return nil
}

func (b *memoryClusterBackend) setFailures(read bool, write bool, lease bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.failRead, b.failWrite, b.failLease = read, write, lease
}

func (b *memoryClusterBackend) setState(key string, state ClusterState) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.states[key] = cloneClusterState(state)
}

func (b *memoryClusterBackend) state(key string) (ClusterState, bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	state, found := b.states[key]
	return cloneClusterState(state), found
}

func (b *memoryClusterBackend) hasLease(key string) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	_, found := b.leases[key]
	return found
}

func (b *memoryClusterBackend) replaceOwner(key string, duration time.Duration) ClusterLease {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.nextFence++
	lease := ClusterLease{
		Key: key, Owner: fmt.Sprintf("replacement-%d", b.nextFence), Fence: b.nextFence,
		ExpireAt: time.Now().Add(duration),
	}
	b.leases[key] = lease
	b.leaseTimes[key] = duration
	return lease
}

func (b *memoryClusterBackend) counts() (reads, advances, acquires, renews, publishes, releases int) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.reads, b.advances, b.acquires, b.renews, b.publishes, b.releases
}

func cloneClusterState(state ClusterState) ClusterState {
	if state.Value == nil {
		return state
	}
	value := *state.Value
	value.Document = append([]byte(nil), value.Document...)
	state.Value = &value
	return state
}

func sameTestLease(left ClusterLease, right ClusterLease) bool {
	return left.Key == right.Key && left.Owner == right.Owner && left.Fence == right.Fence
}
