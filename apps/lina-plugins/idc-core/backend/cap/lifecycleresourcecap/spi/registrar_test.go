// This file verifies single declaration and startup freeze semantics for the lifecycle source.

package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
)

func TestRegistrarAcceptsOneFactoryAndFreezes(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	factory := func(context.Context) (lifecycleresourcecap.Source, error) { return nil, nil }
	if err := r.declare(factory); err != nil {
		t.Fatalf("declare() error = %v", err)
	}
	if r.freeze().Factory() == nil {
		t.Fatal("freeze() omitted the declared factory")
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("declare() accepted a factory after freeze")
	}
}

func TestRegistrarRejectsInvalidOrDuplicateFactory(t *testing.T) {
	t.Parallel()
	factory := func(context.Context) (lifecycleresourcecap.Source, error) { return nil, nil }

	var nilRegistrar *registrar
	if err := nilRegistrar.declare(factory); err == nil {
		t.Fatal("nil registrar accepted a factory")
	}

	r := &registrar{}
	if err := r.declare(nil); err == nil {
		t.Fatal("declare() accepted a nil factory")
	}
	if err := r.declare(factory); err != nil {
		t.Fatalf("first declare() error = %v", err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("declare() accepted a duplicate factory")
	}
}
