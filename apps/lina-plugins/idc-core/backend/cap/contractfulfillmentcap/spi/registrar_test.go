package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/contractfulfillmentcap"
)

type testSource struct{}

func (testSource) BatchEvaluate(context.Context, []contractfulfillmentcap.Requirement) ([]contractfulfillmentcap.Decision, error) {
	return nil, nil
}

func TestRegistrarFreezesExactlyOneFactory(t *testing.T) {
	r := &registrar{}
	factory := func(context.Context) (contractfulfillmentcap.Source, error) { return testSource{}, nil }
	if err := r.declare(factory); err != nil {
		t.Fatalf("declare factory: %v", err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("duplicate factory was accepted")
	}
	if r.freeze().Factory() == nil {
		t.Fatal("declared factory disappeared")
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("factory was accepted after freeze")
	}
}
