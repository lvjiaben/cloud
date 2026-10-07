// This file verifies single-owner declaration and freeze behavior.

package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type inertService struct{}

func (inertService) CurrentMerchant(context.Context) (idccap.MerchantContext, error) {
	return idccap.MerchantContext{}, nil
}

func TestRegistrarAcceptsOneFactoryAndFreezes(t *testing.T) {
	r := &registrar{}
	factory := func(context.Context) (idccap.Service, error) { return inertService{}, nil }
	if err := r.declare(factory); err != nil {
		t.Fatalf("declare factory: %v", err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("duplicate factory declaration succeeded")
	}
	snapshot, err := r.freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze factory: snapshot=%v err=%v", snapshot.Factory(), err)
	}
	if err = r.declare(factory); err == nil {
		t.Fatal("post-freeze declaration succeeded")
	}
}
