// Package spi owns the declaration-time source billing capability factory.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/billingcap"
)

type Factory func(context.Context) (billingcap.Service, error)

type Snapshot struct{ factory Factory }

func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mutex   sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

func Declare(factory Factory) error { return startupRegistrar.declare(factory) }
func Freeze() (Snapshot, error)     { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("billingcap source factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen {
		return gerror.New("billingcap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("billingcap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("billingcap source registrar is unavailable")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("billingcap source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
