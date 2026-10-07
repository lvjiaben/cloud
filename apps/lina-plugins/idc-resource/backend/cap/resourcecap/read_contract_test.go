package resourcecap

import (
	"context"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capregistry"
)

func TestReadDependencyAndProjectionValidation(t *testing.T) {
	dependency, err := NewReadDependency()
	if err != nil || ValidateReadDependency(dependency) != nil {
		t.Fatalf("expected exact read dependency, got %#v err=%v", dependency, err)
	}
	request := ServerCapabilitiesRequest{Routes: []ServerCapabilityRoute{{ServiceNo: "svc-1", ChannelNo: "channel-1", PoolNo: "pool-1"}}}
	if err = ValidateServerCapabilitiesRequest(request); err != nil {
		t.Fatalf("expected valid request: %v", err)
	}
	result := ServerCapabilitiesResult{Items: []ServerCapabilityProjection{{ServiceNo: "svc-1", Capabilities: []CapabilityCode{CapabilityStart, CapabilityStop}, ObservedAt: time.Now().UTC()}}}
	if err = ValidateServerCapabilitiesResult(result, request); err != nil {
		t.Fatalf("expected valid result: %v", err)
	}
	result.Items[0].Capabilities = []CapabilityCode{CapabilityStop, CapabilityStart}
	if ValidateServerCapabilitiesResult(result, request) == nil {
		t.Fatal("expected unsorted capabilities to fail closed")
	}
}

func TestReadDescriptorIsOwnerAwareAndReadOnly(t *testing.T) {
	descriptor, err := NewReadDescriptor(readInvokerStub{})
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.OwnerPluginID != OwnerPluginID || descriptor.Service != ServiceName || descriptor.Version != string(ReadVersion) || len(descriptor.Methods) != 3 {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	method := descriptor.Methods[0]
	if method.Method != string(MethodServerCapabilitiesList) || method.Capability != CapabilityServerCapabilitiesList || method.Risk != "read" {
		t.Fatalf("unexpected method: %#v", method)
	}
	if descriptor.Methods[1].Method != string(MethodCurrentCustomerServersList) ||
		descriptor.Methods[1].Capability != CapabilityCurrentCustomerServersList ||
		descriptor.Methods[2].Method != string(MethodCurrentCustomerServerGet) ||
		descriptor.Methods[2].Capability != CapabilityCurrentCustomerServerGet {
		t.Fatalf("unexpected customer methods: %#v", descriptor.Methods)
	}
}

type readInvokerStub struct{}

func (readInvokerStub) Invoke(_ context.Context, _ capregistry.Invocation) (*capregistry.InvocationResult, error) {
	return &capregistry.InvocationResult{}, nil
}
