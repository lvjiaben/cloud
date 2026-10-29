// Package spi binds a caller-restricted login directory factory at declaration.
package spi

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"sync"
)

type Factory func(context.Context, capability.Services, string) (customerlogincap.Directory, error)

var registry struct {
	sync.RWMutex
	factory Factory
}

func Declare(factory Factory) error {
	if factory == nil {
		return gerror.New("customer login directory factory is required")
	}
	registry.Lock()
	defer registry.Unlock()
	if registry.factory != nil {
		return gerror.New("customer login directory already declared")
	}
	registry.factory = factory
	return nil
}
func Current() Factory { registry.RLock(); defer registry.RUnlock(); return registry.factory }
