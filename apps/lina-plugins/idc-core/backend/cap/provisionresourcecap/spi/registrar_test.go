package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

func TestRegistrarAcceptsOneFactoryAndFreezes(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	factory := func(context.Context) (provisionresourcecap.Source, error) { return nil, nil }
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
