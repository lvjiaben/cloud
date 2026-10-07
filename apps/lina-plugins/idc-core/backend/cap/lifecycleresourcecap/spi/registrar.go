// Package spi owns declaration-time injection of the resource lifecycle source.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
)

// Factory constructs the single resource-owned lifecycle source during startup.
type Factory func(ctx context.Context) (lifecycleresourcecap.Source, error)

// Snapshot is the immutable lifecycle source declaration visible to core.
type Snapshot struct{ factory Factory }

// Factory returns the declared source constructor, or nil when none was declared.
func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mutex   sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

// Declare registers the one resource-owned lifecycle source before startup freeze.
func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

// Freeze seals declarations and returns the immutable startup snapshot.
func Freeze() Snapshot { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("resource lifecycle source factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen {
		return gerror.New("resource lifecycle source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("resource lifecycle source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.frozen = true
	return Snapshot{factory: r.factory}
}
