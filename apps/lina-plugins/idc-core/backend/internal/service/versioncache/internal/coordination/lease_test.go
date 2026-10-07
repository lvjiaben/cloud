package coordination

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestGuardPublishUsesAtomicLeaseAndStatePreconditions(t *testing.T) {
	backend := newAtomicTestBackend()
	controller, err := newController(backend, time.Second, time.Now)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	guard, _, err := controller.Acquire(t.Context(), "catalog")
	if err != nil || guard == nil {
		t.Fatalf("acquire guard=%v err=%v", guard, err)
	}
	defer func() {
		_ = guard.StopAndCheck()
		_ = guard.Release(t.Context())
	}()
	state, err := guard.Publish(t.Context(), "catalog", State{}, "v1", Value{
		Version: "v1", Document: []byte(`{"name":"public"}`), LoadedAt: time.Now(),
	})
	if err != nil || state.Generation != 1 || state.Watermark != "v1" || state.Value == nil {
		t.Fatalf("published state=%+v err=%v", state, err)
	}
	state.Value.Document[0] = 'x'
	stored, found, err := backend.Read(t.Context(), "catalog")
	if err != nil || !found || string(stored.Value.Document) != `{"name":"public"}` {
		t.Fatalf("stored state=%+v found=%v err=%v", stored, found, err)
	}
}

func TestExpiredOrReplacedOwnerCannotPublish(t *testing.T) {
	backend := newAtomicTestBackend()
	controller, err := newController(backend, time.Second, time.Now)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	guard, _, err := controller.Acquire(t.Context(), "catalog")
	if err != nil || guard == nil {
		t.Fatalf("acquire guard=%v err=%v", guard, err)
	}
	backend.replaceOwner("catalog")
	if _, err = guard.Publish(t.Context(), "catalog", State{}, "v1", Value{
		Version: "v1", Document: []byte(`{"name":"must-not-publish"}`), LoadedAt: time.Now(),
	}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("lost-owner publish error=%v, want state changed", err)
	}
	stored, found, readErr := backend.Read(t.Context(), "catalog")
	if readErr != nil || found || stored.Value != nil {
		t.Fatalf("lost owner wrote state=%+v found=%v err=%v", stored, found, readErr)
	}
	_ = guard.StopAndCheck()
	_ = guard.Release(t.Context())
}

func TestGuardTracksAcquireExpiryAndRefusesExpiredPublish(t *testing.T) {
	backend := newAtomicTestBackend()
	now := time.Now()
	controller, err := newController(backend, time.Second, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	guard, _, err := controller.Acquire(t.Context(), "catalog")
	if err != nil || guard == nil {
		t.Fatalf("acquire guard=%v err=%v", guard, err)
	}
	now = now.Add(2 * time.Second)
	if _, err = guard.Publish(t.Context(), "catalog", State{}, "v1", Value{
		Version: "v1", Document: []byte(`{"name":"expired"}`), LoadedAt: now,
	}); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("expired publish error=%v, want lease lost", err)
	}
	stored, found, readErr := backend.Read(t.Context(), "catalog")
	if readErr != nil || found || stored.Value != nil {
		t.Fatalf("expired guard wrote state=%+v found=%v err=%v", stored, found, readErr)
	}
	_ = guard.StopAndCheck()
	_ = guard.Release(t.Context())
}

func TestInvalidationRacePreventsPublish(t *testing.T) {
	backend := newAtomicTestBackend()
	controller, err := newController(backend, time.Second, time.Now)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	guard, _, err := controller.Acquire(t.Context(), "catalog")
	if err != nil || guard == nil {
		t.Fatalf("acquire guard=%v err=%v", guard, err)
	}
	if _, err = controller.Advance(t.Context(), "catalog", "v2"); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if _, err = guard.Publish(t.Context(), "catalog", State{}, "v1", Value{
		Version: "v1", Document: []byte(`{"name":"old"}`), LoadedAt: time.Now(),
	}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("racing publish error=%v, want state changed", err)
	}
	stored, found, readErr := backend.Read(t.Context(), "catalog")
	if readErr != nil || !found || stored.Watermark != "v2" || stored.Value != nil {
		t.Fatalf("invalidation state=%+v found=%v err=%v", stored, found, readErr)
	}
	_ = guard.StopAndCheck()
	_ = guard.Release(t.Context())
}

func TestStopAndCheckCancelsBlockedRenewBeforeWaiting(t *testing.T) {
	entered := make(chan struct{}, 1)
	backend := newAtomicTestBackend()
	backend.renew = func(ctx context.Context, lease Lease) (Lease, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return Lease{}, ctx.Err()
	}
	controller, err := newController(backend, 90*time.Millisecond, time.Now)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	guard, _, err := controller.Acquire(t.Context(), "catalog")
	if err != nil || guard == nil {
		t.Fatalf("acquire guard=%v err=%v", guard, err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("renew did not start")
	}
	returned := make(chan error, 1)
	go func() { returned <- guard.StopAndCheck() }()
	select {
	case err = <-returned:
		if err != nil {
			t.Fatalf("stop guard: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("StopAndCheck waited without canceling blocked Renew")
	}
}

func TestRenewFailureCancelsGuardAndFailsClosed(t *testing.T) {
	backend := newAtomicTestBackend()
	backend.renew = func(context.Context, Lease) (Lease, error) {
		return Lease{}, errors.New("renew unavailable")
	}
	controller, err := newController(backend, 60*time.Millisecond, time.Now)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	guard, _, err := controller.Acquire(t.Context(), "catalog")
	if err != nil || guard == nil {
		t.Fatalf("acquire guard=%v err=%v", guard, err)
	}
	select {
	case <-guard.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("renew failure did not cancel guard")
	}
	if err = guard.StopAndCheck(); err == nil {
		t.Fatal("renew failure was not reported")
	}
	if err = guard.Check(); err == nil {
		t.Fatal("guard remained usable after renew failure")
	}
}

func TestAcquireRejectsExpiredContentionDeadline(t *testing.T) {
	backend := newAtomicTestBackend()
	backend.acquire = func(context.Context, string, time.Duration) (*AcquireOutput, error) {
		return &AcquireOutput{
			Acquired: false,
			Lease:    Lease{Key: "catalog", Owner: "owner", Fence: 1, ExpireAt: time.Now().Add(-time.Second)},
		}, nil
	}
	controller, err := newController(backend, time.Second, time.Now)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	if guard, _, acquireErr := controller.Acquire(t.Context(), "catalog"); acquireErr == nil || guard != nil {
		t.Fatalf("expired contention response guard=%v err=%v", guard, acquireErr)
	}
}

type atomicTestBackend struct {
	mutex   sync.Mutex
	states  map[string]State
	leases  map[string]Lease
	next    uint64
	acquire func(context.Context, string, time.Duration) (*AcquireOutput, error)
	renew   func(context.Context, Lease) (Lease, error)
}

func newAtomicTestBackend() *atomicTestBackend {
	return &atomicTestBackend{states: make(map[string]State), leases: make(map[string]Lease)}
}

func (b *atomicTestBackend) Read(ctx context.Context, key string) (State, bool, error) {
	if err := ctx.Err(); err != nil {
		return State{}, false, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	state, found := b.states[key]
	return CloneState(state), found, nil
}

func (b *atomicTestBackend) Advance(ctx context.Context, key string, watermark string) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	state := b.states[key]
	state.Generation++
	state.Watermark = watermark
	state.Value = nil
	b.states[key] = CloneState(state)
	return CloneState(state), nil
}

func (b *atomicTestBackend) Acquire(
	ctx context.Context,
	key string,
	leaseDuration time.Duration,
) (*AcquireOutput, error) {
	if b.acquire != nil {
		return b.acquire(ctx, key, leaseDuration)
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.next++
	lease := Lease{
		Key: key, Owner: "owner", Fence: b.next, ExpireAt: time.Now().Add(leaseDuration),
	}
	b.leases[key] = lease
	return &AcquireOutput{Acquired: true, Lease: lease}, nil
}

func (b *atomicTestBackend) Renew(ctx context.Context, lease Lease) (Lease, error) {
	if b.renew != nil {
		return b.renew(ctx, lease)
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	held, found := b.leases[lease.Key]
	if !found || !sameLeaseIdentity(held, lease) || !held.ExpireAt.After(time.Now()) {
		return Lease{}, ErrLeaseLost
	}
	held.ExpireAt = time.Now().Add(time.Second)
	b.leases[lease.Key] = held
	return held, nil
}

func (b *atomicTestBackend) Publish(ctx context.Context, input PublishInput) (*PublishOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	held, found := b.leases[input.Key]
	state := b.states[input.Key]
	if !found || !sameLeaseIdentity(held, input.Lease) || !held.ExpireAt.After(time.Now()) ||
		state.Generation != input.ExpectedGeneration || state.Watermark != input.ExpectedWatermark {
		return &PublishOutput{State: CloneState(state)}, nil
	}
	if state.Generation == 0 || state.Watermark != input.AuthoritativeMarker {
		state.Generation++
	}
	value := cloneValue(input.Value)
	state.Watermark = input.AuthoritativeMarker
	state.Value = &value
	b.states[input.Key] = CloneState(state)
	return &PublishOutput{Published: true, State: CloneState(state)}, nil
}

func (b *atomicTestBackend) Release(ctx context.Context, lease Lease) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if held, found := b.leases[lease.Key]; found && sameLeaseIdentity(held, lease) {
		delete(b.leases, lease.Key)
	}
	return nil
}

func (b *atomicTestBackend) replaceOwner(key string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.next++
	b.leases[key] = Lease{
		Key: key, Owner: "replacement", Fence: b.next, ExpireAt: time.Now().Add(time.Second),
	}
}
