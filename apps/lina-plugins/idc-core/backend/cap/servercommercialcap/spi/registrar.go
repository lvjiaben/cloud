// Package spi freezes the single source-only commercial owner at startup.
package spi

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"sync"
)

// Factory receives the resource caller's already constructed source. Neither
// side calls the other factory recursively or constructs services per request.
type Factory func(context.Context, capability.Services, string, serveroperationresourcecap.Source) (servercommercialcap.Service, error)
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
		return gerror.New("server commercial source factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen || r.factory != nil {
		return gerror.New("server commercial source declaration is closed")
	}
	r.factory = factory
	return nil
}
func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("server commercial source registrar is unavailable")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("server commercial source is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
