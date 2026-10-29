package nativecommercial

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

type scanOwner struct {
	servercommercialcap.Service
	seen    []string
	failure error
}

func (o *scanOwner) ReconcileReady(context.Context, int) (int, error) {
	return 0, errors.New("one recovery item failed")
}

func (o *scanOwner) ListReady(context.Context, int) ([]string, error) {
	return []string{"unavailable-server", "healthy-server", "another-server"}, nil
}

func (o *scanOwner) WithReadyOperation(_ context.Context, number string, _ func(context.Context, servercommercialcap.Operation) (string, error)) error {
	o.seen = append(o.seen, number)
	if number == "unavailable-server" {
		return o.failure
	}
	return nil
}

func TestReadyScanContinuesAfterOneServerFails(t *testing.T) {
	owner := &scanOwner{failure: errors.New("server admission unavailable")}
	service := &Service{commercial: owner}
	if err := service.RunReady(context.Background()); !errors.Is(err, owner.failure) {
		t.Fatalf("scan must retain actionable failure: %v", err)
	}
	if !reflect.DeepEqual(owner.seen, []string{"unavailable-server", "healthy-server", "another-server"}) {
		t.Fatalf("one rejected server starved unrelated operations: %v", owner.seen)
	}
}
