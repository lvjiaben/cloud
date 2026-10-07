// Package spi owns declaration-time composition for the source-only identity capability.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// Factory constructs the core-owned identity service for one explicit source caller.
type Factory func(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (identitycap.Runtime, error)

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
		return gerror.New("identitycap source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("identitycap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("identitycap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("identitycap source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("identitycap source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
