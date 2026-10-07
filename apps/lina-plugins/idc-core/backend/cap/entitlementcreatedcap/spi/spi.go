package spi

import (
	"context"
	"errors"
	"strings"
	"sync"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
)

type Factory func(context.Context) (entitlementcreatedcap.Sink, error)

var registry struct {
	sync.RWMutex
	factories map[string]Factory
	frozen    bool
}

func Register(ownerID string, factory Factory) error {
	registry.Lock()
	defer registry.Unlock()
	ownerID = strings.TrimSpace(ownerID)
	if registry.frozen || ownerID == "" || factory == nil {
		return errors.New("entitlement-created owner registration is unavailable")
	}
	if registry.factories == nil {
		registry.factories = make(map[string]Factory)
	}
	if _, exists := registry.factories[ownerID]; exists {
		return errors.New("entitlement-created owner is already registered")
	}
	registry.factories[ownerID] = factory
	return nil
}

type Snapshot struct{ factories map[string]Factory }

func Freeze() Snapshot {
	registry.Lock()
	defer registry.Unlock()
	registry.frozen = true
	copyFactories := make(map[string]Factory, len(registry.factories))
	for ownerID, factory := range registry.factories {
		copyFactories[ownerID] = factory
	}
	return Snapshot{factories: copyFactories}
}

func (snapshot Snapshot) Empty() bool { return len(snapshot.factories) == 0 }

func (snapshot Snapshot) Factory(ownerID string) (Factory, bool) {
	factory, ok := snapshot.factories[ownerID]
	return factory, ok
}

// Build constructs all registered sinks once at startup. A failed owner
// construction fails startup instead of leaving only part of the routing map.
func (snapshot Snapshot) Build(ctx context.Context) (entitlementcreatedcap.Router, error) {
	router := &frozenRouter{sinks: make(map[string]entitlementcreatedcap.Sink, len(snapshot.factories))}
	for ownerID, factory := range snapshot.factories {
		sink, err := factory(ctx)
		if err != nil || sink == nil {
			if err == nil {
				err = errors.New("entitlement-created owner sink is nil")
			}
			return nil, err
		}
		router.sinks[ownerID] = sink
	}
	return router, nil
}

type frozenRouter struct {
	sinks map[string]entitlementcreatedcap.Sink
}

func (router *frozenRouter) Resolve(ownerID string) (entitlementcreatedcap.Sink, bool) {
	if router == nil {
		return nil, false
	}
	sink, ok := router.sinks[ownerID]
	return sink, ok && sink != nil
}
