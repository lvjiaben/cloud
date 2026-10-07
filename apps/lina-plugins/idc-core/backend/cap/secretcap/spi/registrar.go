// Package spi owns the declaration-time source secret factory registrar.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// Factory constructs the core-owned source secret capability during explicit
// consumer startup composition. The returned value remains an in-process
// callback boundary and must not be published as a dynamic plaintext bridge.
type Factory func(ctx context.Context) (secretcap.Source, error)

// Snapshot is the immutable result of freezing the one owner declaration.
type Snapshot struct {
	factory Factory
}

// Factory returns the declared owner factory. It is intentionally available
// only from an already-frozen snapshot and is not a runtime lookup method.
func (s Snapshot) Factory() Factory {
	return s.factory
}

type registrar struct {
	mu      sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

// Declare records the single core owner factory during package initialization.
// Duplicate and post-freeze declarations fail without changing state.
func Declare(factory Factory) error {
	return startupRegistrar.declare(factory)
}

// Freeze permanently closes declaration and returns an immutable snapshot.
// Missing owner declaration fails closed.
func Freeze() (Snapshot, error) {
	return startupRegistrar.freeze()
}

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("secretcap source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("secretcap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("secretcap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("secretcap source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("secretcap source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
