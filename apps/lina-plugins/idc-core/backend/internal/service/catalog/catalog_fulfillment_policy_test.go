package catalog

import (
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestBuildFulfillmentPolicyEvidenceKeepsTechnicalFieldsServerOwned(t *testing.T) {
	t.Parallel()
	manual, err := BuildFulfillmentPolicyEvidence(FulfillmentStrategyManual, nil)
	if err != nil {
		t.Fatalf("build manual fulfillment policy: %v", err)
	}
	if manual.GranularityCode != productfulfillmentcap.GranularityPerOrderItem ||
		manual.ProvisionMethod != "" || manual.QueryMethod != "" ||
		manual.MaximumAttempts != 10 || manual.RetryBaseSeconds != 30 ||
		!fulfillmentPolicyDigestPattern.MatchString(manual.PolicyDigest) {
		t.Fatalf("unexpected manual policy: %+v", manual)
	}

	contract := &FrozenFulfillmentContract{
		Capability:         &capmodel.LocalizedText{ZhCN: "签发证书", EnUS: "Issue certificates"},
		Granularity:        productfulfillmentcap.GranularityPerQuantityUnit,
		OngoingManagement:  true,
		RenewalMode:        productfulfillmentcap.RenewalModeLocalEntitlement,
		EntitlementOwnerID: "ssl-owner",
		ProvisionMethod:    "certificate.issue", QueryMethod: "certificate.query",
		DeclarationDigest: strings.Repeat("a", 64),
	}
	automatic, err := BuildFulfillmentPolicyEvidence(FulfillmentStrategyPaymentConfirmedPlugin, contract)
	if err != nil {
		t.Fatalf("build plugin fulfillment policy: %v", err)
	}
	if automatic.GranularityCode != contract.Granularity || !automatic.OngoingManagement || automatic.ProvisionMethod != contract.ProvisionMethod ||
		automatic.QueryMethod != contract.QueryMethod || automatic.PolicyDigest == manual.PolicyDigest {
		t.Fatalf("unexpected plugin policy: %+v", automatic)
	}
	if err = ValidateFulfillmentPolicyEvidence(automatic, contract); err != nil {
		t.Fatalf("validate unchanged policy: %v", err)
	}
	changed := *contract
	changed.OngoingManagement = false
	changed.RenewalMode = productfulfillmentcap.RenewalModeUnavailable
	if err = ValidateFulfillmentPolicyEvidence(automatic, &changed); !bizerr.Is(err, CodeFulfillmentPolicyContractChanged) {
		t.Fatalf("changed ongoing management declaration accepted: %v", err)
	}
}

func TestBuildFulfillmentPolicyEvidenceRejectsMissingOrMalformedPluginContract(t *testing.T) {
	t.Parallel()
	for name, strategy := range map[string]string{
		"approval": FulfillmentStrategyApprovalThenPlugin,
		"payment":  FulfillmentStrategyPaymentConfirmedPlugin,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildFulfillmentPolicyEvidence(strategy, nil); !bizerr.Is(err, CodeFulfillmentPolicyBindingRequired) {
				t.Fatalf("missing contract returned %v", err)
			}
		})
	}
	if _, err := BuildFulfillmentPolicyEvidence("supplier_method", nil); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatalf("unknown merchant strategy returned %v", err)
	}
}
