package channelcap

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

type inertAdapter struct{}

func (inertAdapter) ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error {
	return nil
}
func (inertAdapter) ValidateDestination([]byte) error                  { return nil }
func (inertAdapter) Deliver(context.Context, Delivery) (Result, error) { return Result{}, nil }
func (inertAdapter) Query(context.Context, Query) (Result, error)      { return Result{}, nil }

func TestValidateDescriptorRejectsUnsafeSchema(t *testing.T) {
	factory := func(secretcap.Accessor) (Adapter, error) { return inertAdapter{}, nil }
	if err := ValidateDescriptor(Descriptor{
		ProviderCode: "provider", Kind: KindEmail, Factory: factory,
		Fields: []Field{{Key: "token", LabelKey: "notify.token", Secret: true, Required: true, MaxLength: 512}},
	}); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
	if err := ValidateDescriptor(Descriptor{
		ProviderCode: "Provider With Spaces", Kind: KindEmail, Factory: factory,
	}); err == nil {
		t.Fatal("unsafe provider code was accepted")
	}
}
