// This file verifies managed billing and explicit-currency schedule validation.

package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestManagedBillingNormalizationReusesAuthoritativeModels(t *testing.T) {
	t.Parallel()
	maximumRenewalCycles := 12
	input := BillingRuleCreateInput{
		VersionNo: "version-draft-1", RuleCode: " Monthly ", Active: true,
		Definition: BillingRuleDefinition{
			ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnitMonth, PeriodCount: 1,
			PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed,
			MinimumPurchaseCycles: 1, MaximumRenewalCycles: &maximumRenewalCycles,
		},
	}
	normalized, err := normalizeBillingRuleCreate(input)
	if err != nil || normalized.ruleCode != "monthly" || normalized.status != catalogStatusActive ||
		normalized.definition.MaximumRenewalCycles == input.Definition.MaximumRenewalCycles {
		t.Fatalf("normalize billing rule=%+v err=%v", normalized, err)
	}
	invalid := input
	invalid.Definition.PeriodUnit = PeriodUnitOnce
	if _, err = normalizeBillingRuleCreate(invalid); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatalf("ambiguous recurring period returned %v", err)
	}
}

func TestManagedPriceScheduleRequiresExactCurrencyAndCompleteBounds(t *testing.T) {
	t.Parallel()
	renewal := int64(10_900)
	input := PriceScheduleCreateInput{
		VersionNo: "version-draft-1", BillingRuleNo: "billing-monthly",
		CurrencyCode: "CNY", Active: true, Tiers: []PriceTierDefinition{{
			MinimumQuantity: 0, AmountMinor: 9_900, RenewalAmountMinor: &renewal,
		}},
	}
	normalized, err := normalizePriceScheduleCreate(input)
	if err != nil || normalized.currencyCode != "CNY" || len(normalized.tiers) != 1 ||
		normalized.tiers[0].RenewalAmountMinor == input.Tiers[0].RenewalAmountMinor {
		t.Fatalf("normalize price schedule=%+v err=%v", normalized, err)
	}
	input.CurrencyCode = "cny"
	if _, err = normalizePriceScheduleCreate(input); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatalf("lowercase currency returned %v", err)
	}
	input.CurrencyCode = " CNY "
	if _, err = normalizePriceScheduleCreate(input); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatalf("fuzzy currency returned %v", err)
	}
	input.CurrencyCode = "CNY"
	input.Tiers = nil
	if _, err = normalizePriceScheduleCreate(input); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatalf("empty price schedule returned %v", err)
	}
}

func TestManagedBillingAuthorizesBeforeValidation(t *testing.T) {
	t.Parallel()
	authorizationError := errors.New("authorization unavailable")
	authorizer := &managementTestAuthorizer{err: authorizationError}
	service, err := newManagementService(
		&managementTestAudit{}, authorizer, strings.NewReader(strings.Repeat("x", 64)),
	)
	if err != nil {
		t.Fatalf("create management service: %v", err)
	}
	if _, err = service.CreateBillingRule(
		context.Background(), BillingRuleCreateInput{VersionNo: "../../unsafe"},
	); !errors.Is(err, authorizationError) {
		t.Fatalf("billing authorization failure returned %v", err)
	}
	if _, err = service.CreatePriceSchedule(
		context.Background(), PriceScheduleCreateInput{VersionNo: "../../unsafe"},
	); !errors.Is(err, authorizationError) {
		t.Fatalf("price authorization failure returned %v", err)
	}
	if authorizer.calls.Load() != 2 {
		t.Fatalf("billing authorization calls=%d want=2", authorizer.calls.Load())
	}
}
