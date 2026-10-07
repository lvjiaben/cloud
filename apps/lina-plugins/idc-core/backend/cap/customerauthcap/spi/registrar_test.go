package spi

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
)

type testReader struct{}

func (testReader) CurrentCustomer(context.Context) (*customerauthcap.Subject, error) {
	return &customerauthcap.Subject{CustomerNo: "customer-1"}, nil
}

func TestRegistrarFreezesOneFactory(t *testing.T) {
	registry := &registrar{}
	factory := func(context.Context, capability.Services, string) (customerauthcap.Reader, error) {
		return testReader{}, nil
	}
	if err := registry.declare(factory); err != nil {
		t.Fatal(err)
	}
	snapshot, err := registry.freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	if err = registry.declare(factory); err == nil {
		t.Fatal("frozen customer source accepted another declaration")
	}
}
