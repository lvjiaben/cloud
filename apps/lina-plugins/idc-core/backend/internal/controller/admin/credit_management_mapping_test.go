// This file verifies canonical credit-management amounts and policy mapping boundaries.

package admin

import (
	"testing"

	"lina-core/pkg/bizerr"
	adminv1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func TestParseCreditAmountEnforcesCanonicalBoundary(t *testing.T) {
	if value, err := parseCreditAmount("0", false); err != nil || value != 0 {
		t.Fatalf("zero late-fee cap must remain supported: value=%d err=%v", value, err)
	}
	for _, value := range []string{"", "-1", "+1", "01", "1.0", "9223372036854775808"} {
		if _, err := parseCreditAmount(value, false); !bizerr.Is(err, billing.CodeCreditInvalid) {
			t.Fatalf("value %q must fail with the credit validation code: %v", value, err)
		}
	}
	if _, err := parseCreditAmount("0", true); !bizerr.Is(err, billing.CodeCreditInvalid) {
		t.Fatalf("positive policy amounts must reject zero: %v", err)
	}
}

func TestManagementCreditPolicyIntentAllowsZeroLateFeeCap(t *testing.T) {
	intent, err := managementCreditPolicyIntent(adminv1.ManagementCreditPolicyIntent{
		RequestNo: "credit-policy-mapping", CurrencyCode: "CNY",
		MaximumLimitMinor: "100000", DualReviewThresholdMinor: "50000",
		BillingDay: 10, RepaymentPeriodDays: 15, GracePeriodDays: 3,
		DailyLateFeeBasisPoints: 5, MaximumLateFeeMinor: "0",
	})
	if err != nil {
		t.Fatalf("map valid policy intent: %v", err)
	}
	if intent.MaximumLimitMinor != 100_000 || intent.DualReviewThresholdMinor != 50_000 || intent.MaximumLateFeeMinor != 0 {
		t.Fatalf("policy minor-unit amounts changed during mapping: %+v", intent)
	}
}

// A new policy no longer requires operators to configure a peer-review threshold.
func TestManagementCreditPolicyIntentAllowsOmittedHistoricReviewThreshold(t *testing.T) {
	intent, err := managementCreditPolicyIntent(adminv1.ManagementCreditPolicyIntent{
		RequestNo: "credit-policy-direct-mapping", CurrencyCode: "CNY",
		MaximumLimitMinor: "100000", BillingDay: 10, RepaymentPeriodDays: 15,
		GracePeriodDays: 3, MaximumLateFeeMinor: "0",
	})
	if err != nil || intent.DualReviewThresholdMinor != intent.MaximumLimitMinor {
		t.Fatalf("optional historic threshold: %+v %v", intent, err)
	}
}
