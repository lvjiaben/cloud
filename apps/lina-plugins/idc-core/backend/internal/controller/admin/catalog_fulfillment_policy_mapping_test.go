// This file validates safe fulfillment capability projections for product editing.

package admin

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestFulfillmentPolicySummaryUsesOnlyReadableCapabilityAndGranularity(t *testing.T) {
	t.Parallel()
	policy := &catalog.FulfillmentPolicyEvidence{
		StrategyCode:    catalog.FulfillmentStrategyApprovalThenPlugin,
		GranularityCode: productfulfillmentcap.GranularityPerQuantityUnit,
		ProvisionMethod: "phone.ship", QueryMethod: "phone.track",
		MaximumAttempts: 10, RetryBaseSeconds: 30,
	}
	contract := &catalog.FrozenFulfillmentContract{
		Capability: &capmodel.LocalizedText{ZhCN: "手机发货与物流跟踪", EnUS: "Phone shipping and tracking"},
	}
	result := mapFulfillmentPolicySummary(policy, contract)
	if result == nil || result.CapabilityZhCn != "手机发货与物流跟踪" ||
		result.QuantityHandling != "separately" || result.StrategyCode != policy.StrategyCode {
		t.Fatalf("unexpected safe fulfillment summary: %+v", result)
	}
	manual := mapFulfillmentPolicySummary(&catalog.FulfillmentPolicyEvidence{
		StrategyCode:    catalog.FulfillmentStrategyManual,
		GranularityCode: productfulfillmentcap.GranularityPerOrderItem,
	}, contract)
	if manual.CapabilityZhCn != "" || manual.QuantityHandling != "together" {
		t.Fatalf("manual summary leaked plugin capability or lost quantity description: %+v", manual)
	}
}

func TestTrialCapabilityRequiresExplicitAutomaticFulfillment(t *testing.T) {
	t.Parallel()
	for _, item := range []struct {
		name     string
		strategy string
		contract *catalog.FrozenFulfillmentContract
		want     bool
	}{
		{"unbound", catalog.FulfillmentStrategyPaymentConfirmedPlugin, nil, false},
		{"undeclared", catalog.FulfillmentStrategyPaymentConfirmedPlugin, &catalog.FrozenFulfillmentContract{}, false},
		{"manual", catalog.FulfillmentStrategyManual, &catalog.FrozenFulfillmentContract{TrialPauseResume: true}, false},
		{"automatic", catalog.FulfillmentStrategyPaymentConfirmedPlugin, &catalog.FrozenFulfillmentContract{TrialPauseResume: true}, true},
		{"approval", catalog.FulfillmentStrategyApprovalThenPlugin, &catalog.FrozenFulfillmentContract{TrialPauseResume: true}, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			got := mapFulfillmentPolicySummary(&catalog.FulfillmentPolicyEvidence{StrategyCode: item.strategy}, item.contract)
			if got.TrialSupported != item.want {
				t.Fatalf("trial supported=%v want %v", got.TrialSupported, item.want)
			}
		})
	}
}
