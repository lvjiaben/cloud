package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/poolsourcecap"
)

func TestRegistrarAcceptsOneFactoryAndFreezes(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	factory := func(context.Context) (poolsourcecap.Source, error) { return nil, nil }
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

func TestRegistrarRejectsMissingAndDuplicateFactory(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	if err := r.declare(nil); err == nil {
		t.Fatal("declare() accepted a nil factory")
	}
	factory := func(context.Context) (poolsourcecap.Source, error) { return nil, nil }
	if err := r.declare(factory); err != nil {
		t.Fatalf("declare() error = %v", err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("declare() accepted a duplicate factory")
	}
}
