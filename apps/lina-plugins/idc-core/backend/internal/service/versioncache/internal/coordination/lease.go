// This file manages renewable fenced leases and guarantees cancellation is
// signaled before callers wait for a blocked Renew operation to return.

package coordination

import (
	"context"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Controller applies finite lease and release bounds around a Backend.
type Controller struct {
	backend Backend
	lease   time.Duration
	now     func() time.Time
}

// New constructs a coordination controller. backend must be a shared,
// persistent implementation of the atomic Backend contract.
func New(backend Backend, lease time.Duration) (*Controller, error) {
	return newController(backend, lease, time.Now)
}

func newController(backend Backend, lease time.Duration, now func() time.Time) (*Controller, error) {
	if backend == nil || lease <= 0 || now == nil {
		return nil, gerror.New("version cache coordination requires backend, lease, and clock")
	}
	return &Controller{backend: backend, lease: lease, now: now}, nil
}

// Read returns detached durable shared state.
func (c *Controller) Read(ctx context.Context, key string) (State, bool, error) {
	state, found, err := c.backend.Read(ctx, key)
	return CloneState(state), found, err
}

// Advance publishes a durable invalidation watermark and clears the payload.
func (c *Controller) Advance(ctx context.Context, key string, watermark string) (State, error) {
	state, err := c.backend.Advance(ctx, key, watermark)
	return CloneState(state), err
}

// Guard owns one renewable fenced lease and a context canceled on lease loss.
type Guard struct {
	controller *Controller
	// The guard owns this derived lifetime context for renewal and publication.
	ctx      context.Context //nolint:containedctx // Context lifetime is the Guard's explicit responsibility.
	cancel   context.CancelFunc
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	mutex    sync.Mutex
	lease    Lease
	err      error
	stopping bool
}

// Acquire obtains and starts renewing one fenced lease. On contention the
// returned guard is nil and contentionExpiry is the current owner's deadline.
func (c *Controller) Acquire(
	ctx context.Context,
	key string,
) (guard *Guard, contentionExpiry time.Time, err error) {
	output, err := c.backend.Acquire(ctx, key, c.lease)
	if err != nil {
		return nil, time.Time{}, err
	}
	if output == nil || !validLeaseShape(output.Lease, key) {
		return nil, time.Time{}, gerror.New("version cache backend returned an invalid lease")
	}
	if !output.Acquired {
		if !output.Lease.ExpireAt.After(c.now()) {
			return nil, time.Time{}, gerror.New("version cache backend returned an expired contention lease")
		}
		return nil, output.Lease.ExpireAt, nil
	}
	if output.Lease.Owner == "" || output.Lease.Fence == 0 || !output.Lease.ExpireAt.After(c.now()) {
		return nil, time.Time{}, gerror.New("version cache backend returned an invalid acquired lease")
	}
	guardCtx, cancel := context.WithCancel(ctx)
	guard = &Guard{
		controller: c,
		ctx:        guardCtx,
		cancel:     cancel,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		lease:      output.Lease,
	}
	go guard.renew()
	return guard, time.Time{}, nil
}

// Context is canceled when the request ends, renewal fails, or StopAndCheck
// begins stopping the guard.
func (g *Guard) Context() context.Context {
	return g.ctx
}

// Check fails closed when renewal failed or the latest known lease expiry has
// passed. Atomic Publish remains the final ownership check.
func (g *Guard) Check() error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.err != nil {
		return g.err
	}
	if !g.lease.ExpireAt.After(g.controller.now()) {
		return ErrLeaseLost
	}
	return g.ctx.Err()
}

// Publish delegates the final write to Backend.Publish, where lease ownership,
// expiry, fence, generation, and watermark are checked atomically.
func (g *Guard) Publish(
	ctx context.Context,
	key string,
	expected State,
	watermark string,
	value Value,
) (State, error) {
	if err := g.Check(); err != nil {
		return State{}, err
	}
	g.mutex.Lock()
	lease := g.lease
	g.mutex.Unlock()
	output, err := g.controller.backend.Publish(ctx, PublishInput{
		Key:                 key,
		Lease:               lease,
		ExpectedGeneration:  expected.Generation,
		ExpectedWatermark:   expected.Watermark,
		AuthoritativeMarker: watermark,
		Value:               cloneValue(value),
	})
	if err != nil {
		return State{}, err
	}
	if output == nil {
		return State{}, gerror.New("version cache backend returned no publish result")
	}
	if !output.Published {
		return CloneState(output.State), ErrStateChanged
	}
	return CloneState(output.State), nil
}

// StopAndCheck first cancels the renewal context, then waits for Renew to
// return. Backends are contractually required to honor that cancellation.
func (g *Guard) StopAndCheck() error {
	g.once.Do(func() {
		g.mutex.Lock()
		g.stopping = true
		g.mutex.Unlock()
		g.cancel()
		close(g.stop)
	})
	<-g.done
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.err
}

// Release releases the latest lease using a cancellation-independent finite
// context. It is safe to call after StopAndCheck.
func (g *Guard) Release(ctx context.Context) error {
	releaseTimeout := g.controller.lease / 4
	if releaseTimeout <= 0 || releaseTimeout > 2*time.Second {
		releaseTimeout = 2 * time.Second
	}
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()
	g.mutex.Lock()
	lease := g.lease
	g.mutex.Unlock()
	return g.controller.backend.Release(releaseCtx, lease)
}

func (g *Guard) renew() {
	defer close(g.done)
	interval := g.controller.lease / 3
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-g.ctx.Done():
			if !g.isStopping() {
				g.fail(ErrLeaseLost)
			}
			return
		case <-ticker.C:
			if err := g.renewOnce(interval); err != nil {
				if !g.isStopping() {
					g.fail(err)
				}
				g.cancel()
				return
			}
		}
	}
}

func (g *Guard) renewOnce(timeout time.Duration) error {
	g.mutex.Lock()
	lease := g.lease
	g.mutex.Unlock()
	if !lease.ExpireAt.After(g.controller.now()) {
		return ErrLeaseLost
	}
	renewCtx, cancel := context.WithTimeout(g.ctx, timeout)
	defer cancel()
	renewed, err := g.controller.backend.Renew(renewCtx, lease)
	if err != nil {
		return gerror.Wrap(err, "renew version cache fenced lease")
	}
	if !sameLeaseIdentity(renewed, lease) || !renewed.ExpireAt.After(g.controller.now()) {
		return ErrLeaseLost
	}
	g.mutex.Lock()
	g.lease = renewed
	g.mutex.Unlock()
	return nil
}

func (g *Guard) isStopping() bool {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.stopping
}

func (g *Guard) fail(err error) {
	if err == nil {
		return
	}
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.err == nil {
		g.err = err
	}
}

func validLeaseShape(lease Lease, key string) bool {
	return lease.Key == key && !lease.ExpireAt.IsZero()
}

func sameLeaseIdentity(left Lease, right Lease) bool {
	return left.Key == right.Key && left.Owner == right.Owner && left.Fence == right.Fence
}

func cloneValue(value Value) Value {
	value.Document = append([]byte(nil), value.Document...)
	return value
}
