// Package spi owns declaration-time composition for customer onboarding.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/onboardingcap"
)

type Factory func(context.Context, capability.Services, string) (onboardingcap.Service, error)
type Snapshot struct{ factory Factory }

func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mu      sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

func Declare(factory Factory) error { return startupRegistrar.declare(factory) }
func Freeze() (Snapshot, error)     { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("onboardingcap source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("onboardingcap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("onboardingcap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("onboardingcap source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("onboardingcap source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
