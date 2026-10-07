// Package spi owns startup composition for the source-only customer reader.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
)

// Factory constructs the bounded reader for one explicit source caller.
type Factory func(context.Context, capability.Services, string) (customerauthcap.Reader, error)

// Snapshot is the immutable result of freezing the single owner declaration.
type Snapshot struct{ factory Factory }

func (snapshot Snapshot) Factory() Factory { return snapshot.factory }

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
		return gerror.New("current customer source factory is required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.frozen {
		return gerror.New("current customer source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("current customer source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("current customer source registrar is unavailable")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.frozen = true
	if r.factory == nil {
		return Snapshot{}, gerror.New("current customer source factory is not declared")
	}
	return Snapshot{factory: r.factory}, nil
}
