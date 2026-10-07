package spi

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

func TestRegistrarAcceptsOneFactoryAndFreezes(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	factory := func(context.Context, capability.Services) (managementcap.Runtime, error) {
		return managementcap.Runtime{}, nil
	}
	if err := r.declare(factory); err != nil {
		t.Fatalf("declare() error = %v", err)
	}
	if snapshot, err := r.freeze(); err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze() snapshot=%+v error=%v", snapshot, err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("declare() accepted a factory after freeze")
	}
}

func TestRegistrarRejectsMissingAndDuplicateFactory(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	if err := r.declare(nil); err == nil {
		t.Fatal("declare() accepted nil factory")
	}
	factory := func(context.Context, capability.Services) (managementcap.Runtime, error) {
		return managementcap.Runtime{}, nil
	}
	if err := r.declare(factory); err != nil {
		t.Fatalf("declare() error = %v", err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("declare() accepted duplicate factory")
	}
}
