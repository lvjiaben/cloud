// Package spi owns startup declaration for the source-only contract snapshot capability.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/contractcap"
)

// Factory builds the core-owned snapshot service for one explicit source caller.
type Factory func(context.Context, capability.Services, string) (contractcap.Runtime, error)

// Snapshot is the frozen owner declaration.
type Snapshot struct{ factory Factory }

// Factory returns the immutable owner factory.
func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mu      sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

// Declare records the single owner factory.
func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

// Freeze closes declaration and returns the owner snapshot.
func Freeze() (Snapshot, error) { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("contractcap source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("contractcap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("contractcap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("contractcap source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("contractcap source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
