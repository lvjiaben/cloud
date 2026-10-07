package spi

import (
	"context"
	"errors"
	"strings"
	"sync"

	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

type Factory func(context.Context) (renewalownercap.Source, error)

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
		return errors.New("renewal owner registration is unavailable")
	}
	if registry.factories == nil {
		registry.factories = make(map[string]Factory)
	}
	if _, exists := registry.factories[ownerID]; exists {
		return errors.New("renewal owner is already registered")
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

func (snapshot Snapshot) Build(ctx context.Context) (renewalownercap.Router, error) {
	router := &frozenRouter{sources: make(map[string]renewalownercap.Source, len(snapshot.factories))}
	for ownerID, factory := range snapshot.factories {
		source, err := factory(ctx)
		if err != nil || source == nil {
			if err == nil {
				err = errors.New("renewal owner source is nil")
			}
			return nil, err
		}
		router.sources[ownerID] = source
	}
	return router, nil
}

type frozenRouter struct {
	sources map[string]renewalownercap.Source
}

func (router *frozenRouter) Resolve(ownerID string) (renewalownercap.Source, bool) {
	if router == nil {
		return nil, false
	}
	source, ok := router.sources[ownerID]
	return source, ok && source != nil
}
