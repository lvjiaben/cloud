package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

type sourceStub struct{}

func (sourceStub) CreateTask(context.Context, renewalownercap.TaskRequest) (*renewalownercap.Task, error) {
	return nil, nil
}
func (sourceStub) ReadTask(context.Context, renewalownercap.TaskReadRequest) (*renewalownercap.Task, error) {
	return nil, nil
}

func TestRegistryRoutesExactFrozenOwners(t *testing.T) {
	factory := func(context.Context) (renewalownercap.Source, error) { return sourceStub{}, nil }
	if err := Register("idc-resource", factory); err != nil {
		t.Fatal(err)
	}
	if err := Register("ssl-owner", factory); err != nil {
		t.Fatal(err)
	}
	if err := Register("idc-resource", factory); err == nil {
		t.Fatal("duplicate renewal owner registration was accepted")
	}
	router, err := Freeze().Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := router.Resolve("idc-resource"); !ok {
		t.Fatal("server renewal owner was not resolved")
	}
	if _, ok := router.Resolve("server-connector"); ok {
		t.Fatal("connector ID was incorrectly treated as the renewal owner")
	}
	if err := Register("late-owner", factory); err == nil {
		t.Fatal("late renewal owner registration was accepted")
	}
}
