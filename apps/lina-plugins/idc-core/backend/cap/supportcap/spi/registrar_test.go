package spi

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/supportcap"
)

type inertSupportService struct{}

func (inertSupportService) CreateAnonymousPresales(context.Context, supportcap.CreateAnonymousPresalesRequest) (*supportcap.CreateAnonymousPresalesResult, error) {
	return nil, nil
}

func (inertSupportService) RenewAnonymousPresalesAccess(context.Context, supportcap.RenewAnonymousPresalesAccessRequest) (*supportcap.RenewAnonymousPresalesAccessResult, error) {
	return nil, nil
}

func TestRegistrarFreezesExactlyOneFactory(t *testing.T) {
	r := &registrar{}
	factory := func(context.Context, capability.Services, string) (supportcap.Service, error) {
		return inertSupportService{}, nil
	}
	if err := r.declare(factory); err != nil {
		t.Fatal(err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("duplicate factory accepted")
	}
	snapshot, err := r.freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze failed: snapshot=%+v err=%v", snapshot, err)
	}
	if err = r.declare(factory); err == nil {
		t.Fatal("declaration after freeze accepted")
	}
}
