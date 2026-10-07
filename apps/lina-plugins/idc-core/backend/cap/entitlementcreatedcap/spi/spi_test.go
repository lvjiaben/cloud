package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
)

type testSink struct{}

func (testSink) Deliver(context.Context, entitlementcreatedcap.Event) (*entitlementcreatedcap.Receipt, error) {
	return nil, nil
}

func TestRegistryFreezesMultipleExactOwners(t *testing.T) {
	factory := func(context.Context) (entitlementcreatedcap.Sink, error) { return testSink{}, nil }
	if err := Register("idc-resource", factory); err != nil {
		t.Fatal(err)
	}
	if err := Register("ssl-owner", factory); err != nil {
		t.Fatal(err)
	}
	if err := Register("idc-resource", factory); err == nil {
		t.Fatal("duplicate business owner registration was accepted")
	}
	snapshot := Freeze()
	router, err := snapshot.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := router.Resolve("idc-resource"); !ok {
		t.Fatal("server business owner was not routed")
	}
	if _, ok := router.Resolve("ssl-owner"); !ok {
		t.Fatal("SSL business owner was not routed")
	}
	if _, ok := router.Resolve("server-connector"); ok {
		t.Fatal("connector plugin ID was incorrectly treated as a business owner")
	}
	if err := Register("late-owner", factory); err == nil {
		t.Fatal("registration after Freeze was accepted")
	}
}
