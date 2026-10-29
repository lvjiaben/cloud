// This file guards the descriptor-only lifecycle policy and fail-closed inputs.

package paymentcap

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
)

// TestBusinessImpactDescriptorOnly forces a lifecycle reassessment when the
// payment guest contract gains any method that could own execution or resources.
func TestBusinessImpactDescriptorOnly(t *testing.T) {
	methods := DynamicProviderMethods()
	if len(methods) != 1 || methods[0] != DynamicProviderMethodDescribe {
		t.Fatal("payment guest execution contract changed; reassess lifecycle impact")
	}
	items, err := BusinessImpact(context.Background(), pluginhost.PluginBusinessImpactInput{TargetPluginID: "third-party-payment"})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("descriptor-only guest acquired transaction references: %+v %v", items, err)
	}
}

// TestBusinessImpactRejectsInvalidContext verifies that malformed calls never
// become a successful empty impact report.
func TestBusinessImpactRejectsInvalidContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		target string
	}{
		{"nil context", nil, "payment-guest"},
		{"blank target", context.Background(), " "},
		{"canceled", canceled, "payment-guest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BusinessImpact(tc.ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: tc.target}); err == nil {
				t.Fatal("unsafe impact request accepted")
			}
		})
	}
}
