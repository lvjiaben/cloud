// Package spi owns declaration-time composition for the source-only IDC base capability.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

// Factory constructs the core-owned IDC service for explicit source-plugin injection.
type Factory func(context.Context) (idccap.Service, error)

// Snapshot is the immutable result of freezing the single owner declaration.
type Snapshot struct{ factory Factory }

// Factory returns the declared owner factory from an already-frozen snapshot.
func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mu      sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

// Declare records the single core owner factory during package initialization.
func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

// Freeze permanently closes declaration and returns an immutable snapshot.
func Freeze() (Snapshot, error) { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("idccap source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("idccap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("idccap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("idccap source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("idccap source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
