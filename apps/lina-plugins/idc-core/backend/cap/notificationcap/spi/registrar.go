// Package spi owns declaration-time composition for the source-only
// notification capability.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

type Factory func(
	context.Context,
	capability.Services,
	string,
) (notificationcap.Runtime, error)

type EventSinkFactory func(
	context.Context,
	capability.Services,
	string,
) (notificationcap.EventSink, error)

type Snapshot struct {
	factory          Factory
	eventSinkFactory EventSinkFactory
}

func (s Snapshot) Factory() Factory { return s.factory }

func (s Snapshot) EventSinkFactory() EventSinkFactory { return s.eventSinkFactory }

type registrar struct {
	mu               sync.Mutex
	frozen           bool
	factory          Factory
	eventSinkFactory EventSinkFactory
}

var startupRegistrar = &registrar{}

func Declare(factory Factory) error { return startupRegistrar.declare(factory) }

func DeclareEventSink(factory EventSinkFactory) error {
	return startupRegistrar.declareEventSink(factory)
}

func Freeze() (Snapshot, error) { return startupRegistrar.freeze() }

func (r *registrar) declare(factory Factory) error {
	if r == nil || factory == nil {
		return gerror.New("notificationcap source factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("notificationcap source registrar is frozen")
	}
	if r.factory != nil {
		return gerror.New("notificationcap source factory is already declared")
	}
	r.factory = factory
	return nil
}

func (r *registrar) declareEventSink(factory EventSinkFactory) error {
	if r == nil || factory == nil {
		return gerror.New("notificationcap event sink factory is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("notificationcap source registrar is frozen")
	}
	if r.eventSinkFactory != nil {
		return gerror.New("notificationcap event sink factory is already declared")
	}
	r.eventSinkFactory = factory
	return nil
}

func (r *registrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("notificationcap source registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	if r.factory == nil || r.eventSinkFactory == nil {
		return Snapshot{}, gerror.New("notificationcap source factory is not declared")
	}
	return Snapshot{factory: r.factory, eventSinkFactory: r.eventSinkFactory}, nil
}
