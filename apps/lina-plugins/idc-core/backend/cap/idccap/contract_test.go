// This file verifies idccap compatibility and descriptor boundaries fail closed.

package idccap

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/capability/capregistry"
)

// TestDependencyCompatibilityFailsClosed verifies identity, version, and
// method mismatches never fall back to the current contract.
func TestDependencyCompatibilityFailsClosed(t *testing.T) {
	valid, err := NewDependency()
	if err != nil {
		t.Fatalf("create dependency: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Dependency)
	}{
		{name: "owner", mutate: func(value *Dependency) { value.OwnerPluginID = "other-owner" }},
		{name: "service", mutate: func(value *Dependency) { value.Service = "other-service" }},
		{name: "version", mutate: func(value *Dependency) { value.Version = "v2" }},
		{name: "missing methods", mutate: func(value *Dependency) { value.Methods = nil }},
		{name: "unknown method", mutate: func(value *Dependency) { value.Methods = []Method{"unknown"} }},
		{name: "duplicate method", mutate: func(value *Dependency) {
			value.Methods = []Method{MethodMerchantContextCurrent, MethodMerchantContextCurrent}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Methods = append([]Method(nil), valid.Methods...)
			test.mutate(&candidate)
			if err := ValidateDependency(candidate); err == nil {
				t.Fatalf("expected incompatible %s dependency to fail", test.name)
			}
		})
	}
}

// TestSupportedMethodsReturnsDetachedSnapshot verifies callers cannot mutate
// compatibility metadata retained by the package.
func TestSupportedMethodsReturnsDetachedSnapshot(t *testing.T) {
	methods := SupportedMethods()
	methods[0] = "mutated"
	if SupportedMethods()[0] != MethodMerchantContextCurrent {
		t.Fatal("expected supported method snapshot to be detached")
	}
}

// TestDescriptorRequiresExplicitInvoker verifies the helper creates metadata
// without registering or locating an implementation.
func TestDescriptorRequiresExplicitInvoker(t *testing.T) {
	if _, err := NewDescriptor(nil); err == nil {
		t.Fatal("expected nil invoker to fail")
	}
	descriptor, err := NewDescriptor(testIDCInvoker{})
	if err != nil {
		t.Fatalf("create descriptor: %v", err)
	}
	if descriptor.OwnerPluginID != OwnerPluginID ||
		descriptor.Service != ServiceName ||
		descriptor.Version != string(ContractVersion) {
		t.Fatalf("unexpected descriptor identity: %#v", descriptor)
	}
	registry := capregistry.NewRegistry()
	if _, ok := registry.LookupDescriptor(OwnerPluginID, ServiceName, string(ContractVersion)); ok {
		t.Fatal("descriptor helper must not register globally")
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("explicit descriptor registration should validate: %v", err)
	}
}

// TestMerchantContextValidationPreservesSingleMerchantSemantics verifies the
// task 2.3 tenant boundary remains exact.
func TestMerchantContextValidationPreservesSingleMerchantSemantics(t *testing.T) {
	current := MerchantContext{
		TenantID:        DefaultTenantID,
		MerchantNo:      "MCH_TEST",
		DefaultLocale:   "zh-CN",
		DefaultCurrency: "CNY",
		Timezone:        "Asia/Shanghai",
		Enabled:         true,
		Version:         1,
	}
	if err := ValidateMerchantContext(current); err != nil {
		t.Fatalf("validate merchant context: %v", err)
	}
	current.TenantID = 1
	if err := ValidateMerchantContext(current); err == nil {
		t.Fatal("expected non-default tenant to fail")
	}
}

type testIDCInvoker struct{}

func (testIDCInvoker) Invoke(context.Context, capregistry.Invocation) (*capregistry.InvocationResult, error) {
	return &capregistry.InvocationResult{}, nil
}
