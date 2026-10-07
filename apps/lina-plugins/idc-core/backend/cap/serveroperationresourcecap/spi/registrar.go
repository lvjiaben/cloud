// Package spi owns declaration-time injection of the server operation source.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type Factory func(context.Context) (serveroperationresourcecap.Source, error)
type Snapshot struct{ factory Factory }

func (s Snapshot) Factory() Factory { return s.factory }

type registrar struct {
	mutex   sync.Mutex
	frozen  bool
	factory Factory
}

var startupRegistrar = &registrar{}

func Declare(factory Factory) error { return startupRegistrar.declare(factory) }
func Freeze() Snapshot              { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("server operation source factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen {
		return gerror.New("server operation source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("server operation source factory is already declared")
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
