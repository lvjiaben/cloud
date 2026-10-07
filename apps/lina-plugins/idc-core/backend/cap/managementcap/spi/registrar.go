// Package spi owns declaration-time construction of the core administration runtime for IDC source plugins.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

// Factory constructs a requester-scoped runtime from host-published ordinary capabilities.
type Factory func(context.Context, capability.Services) (managementcap.Runtime, error)

// Snapshot is the immutable result of declaration freeze.
type Snapshot struct{ factory Factory }

// Factory returns the one core-owned runtime factory.
func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mutex   sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

// Declare records the one core-owned factory during package initialization.
func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

// Freeze permanently closes declaration and returns its immutable snapshot.
func Freeze() (Snapshot, error) { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("IDC management runtime factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen {
		return gerror.New("IDC management runtime registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("IDC management runtime factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("IDC management runtime registrar is unavailable")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("IDC management runtime factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
