// Package spi owns declaration-time injection of the resource-owned pool candidate source.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/poolsourcecap"
)

// Factory creates the startup-scoped resource-owner source.
type Factory func(context.Context) (poolsourcecap.Source, error)

// Snapshot is the immutable result of the one-way declaration freeze.
type Snapshot struct{ factory Factory }

// Factory returns the declared owner factory, or nil when the owner plugin is absent.
func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mutex   sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

// Declare records the one resource-owner factory during package initialization.
func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

// Freeze permanently closes declaration and returns a detached snapshot.
func Freeze() Snapshot { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("resource pool candidate source factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen {
		return gerror.New("resource pool candidate source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("resource pool candidate source factory is already declared")
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
