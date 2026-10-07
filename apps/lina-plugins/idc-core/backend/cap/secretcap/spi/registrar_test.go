package spi

import (
	"context"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

type inertSource struct{}

func (inertSource) Accessor(secretcap.Scope) (secretcap.Accessor, error) { return nil, nil }
func (inertSource) Manager(secretcap.Scope) (secretcap.Manager, error)   { return nil, nil }

func inertFactory(context.Context) (secretcap.Source, error) { return inertSource{}, nil }

func TestRegistrarFreezesOneOwnerFactory(t *testing.T) {
	r := &registrar{}
	if err := r.declare(inertFactory); err != nil {
		t.Fatalf("declare source factory: %v", err)
	}
	if err := r.declare(inertFactory); err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("duplicate declaration error=%v", err)
	}
	snapshot, err := r.freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze source factory: snapshot=%v err=%v", snapshot.Factory(), err)
	}
	if _, err = r.freeze(); err != nil {
		t.Fatalf("repeat freeze: %v", err)
	}
	if err = r.declare(inertFactory); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("post-freeze declaration error=%v", err)
	}
}

func TestRegistrarFailsClosedWithoutOwner(t *testing.T) {
	r := &registrar{}
	if _, err := r.freeze(); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("missing declaration error=%v", err)
	}
	if err := r.declare(inertFactory); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("declare after failed freeze error=%v", err)
	}
}
