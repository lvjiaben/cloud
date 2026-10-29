// Package spi binds the startup-created notification verifier to profile guards.
// The registry stores a service reference, never credential or permission state.
package spi

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"sync"
)

var binding struct {
	sync.RWMutex
	provider profilechangecap.Provider
}

// Bind publishes the startup-created provider. Runtime enablement is checked by
// the consuming guard on every write, including after a provider is disabled.
func Bind(provider profilechangecap.Provider) error {
	if provider == nil {
		return gerror.New("profile change provider is required")
	}
	binding.Lock()
	defer binding.Unlock()
	binding.provider = provider
	return nil
}

// Current returns the already-created provider or nil before route assembly.
func Current() profilechangecap.Provider {
	binding.RLock()
	defer binding.RUnlock()
	return binding.provider
}
