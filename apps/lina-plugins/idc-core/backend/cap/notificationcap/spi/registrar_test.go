package spi

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestRegistrarRejectsDuplicateAndPostFreezeDeclaration(t *testing.T) {
	r := &registrar{}
	factory := func(
		context.Context,
		capability.Services,
		string,
	) (notificationcap.Runtime, error) {
		return notificationcap.Runtime{}, nil
	}
	eventSinkFactory := func(
		context.Context,
		capability.Services,
		string,
	) (notificationcap.EventSink, error) {
		return nil, nil
	}
	if err := r.declare(factory); err != nil {
		t.Fatal(err)
	}
	if err := r.declareEventSink(eventSinkFactory); err != nil {
		t.Fatal(err)
	}
	if err := r.declareEventSink(eventSinkFactory); err == nil {
		t.Fatal("duplicate event sink declaration accepted")
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("duplicate declaration accepted")
	}
	if _, err := r.freeze(); err != nil {
		t.Fatal(err)
	}
	if err := r.declare(factory); err == nil {
		t.Fatal("post-freeze declaration accepted")
	}
	if err := r.declareEventSink(eventSinkFactory); err == nil {
		t.Fatal("post-freeze event sink declaration accepted")
	}
}
