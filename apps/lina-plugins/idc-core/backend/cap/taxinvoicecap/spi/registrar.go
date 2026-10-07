// Package spi owns startup declaration for the source-only tax-invoice capability.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// Factory constructs the source runtime for one explicitly allowed caller.
type Factory func(context.Context, capability.Services, string) (taxinvoicecap.Runtime, error)

// Snapshot freezes the startup declaration before a consumer loads it.
type Snapshot struct{ factory Factory }

// Factory returns the immutable declared runtime factory.
func (s Snapshot) Factory() Factory { return s.factory }

// registrar stores the single startup declaration and its frozen state.
type registrar struct {
	mu      sync.Mutex
	frozen  bool
	factory Factory
}

// startupRegistrar is written only during startup and frozen before loading.
var startupRegistrar = &registrar{}

// Declare publishes the core-owned source factory during startup.
func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

// Freeze prevents later replacement and returns the declared factory snapshot.
func Freeze() (Snapshot, error) { return startupRegistrar.freeze() }

// declare validates and records the only supported factory.
func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("tax invoice source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("tax invoice source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("tax invoice source factory is already declared")
	}
	r.factory = factory
	return nil
}

// freeze makes the registrar immutable and returns its current declaration.
func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("tax invoice source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("tax invoice source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
