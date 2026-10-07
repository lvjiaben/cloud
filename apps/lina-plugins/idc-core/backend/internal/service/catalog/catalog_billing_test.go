package catalog

import (
	"math"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestValidateBillingRuleCoversExplicitPeriodsAndModels(t *testing.T) {
	t.Parallel()
	maximumRenewal := 10
	collectionCount := 1
	roundingQuantity := int64(10)
	minimumUsage := int64(0)
	maximumUsage := int64(10_000)
	cases := []BillingRuleDefinition{
		newFixedBillingRule(ChargeTypeOneTime, PeriodUnitOnce, 1, PricePhaseNewPurchase),
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitHour, 87_600, PricePhaseBoth),
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitDay, 3_650, PricePhaseBoth),
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitMonth, 120, PricePhaseBoth),
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitYear, 10, PricePhaseBoth),
		{
			ChargeType: ChargeTypeTiered, PeriodUnit: PeriodUnitMonth, PeriodCount: 1,
			PricePhase: PricePhaseBoth, QuantityMode: QuantityModeTiered,
			MinimumPurchaseCycles: 1, MaximumRenewalCycles: &maximumRenewal,
			TierMode: "graduated",
		},
		{
			ChargeType: ChargeTypeUsage, PeriodUnit: PeriodUnitMonth, PeriodCount: 1,
			PricePhase: PricePhaseBoth, QuantityMode: QuantityModeUsage,
			MinimumPurchaseCycles: 1, MaximumRenewalCycles: &maximumRenewal,
			MeterUnit: "traffic.gib", TierMode: "graduated",
			UsageCollectionUnit: PeriodUnitHour, UsageCollectionCount: &collectionCount,
			UsageRoundingMode: RoundingModeUp, UsageRoundingQuantity: &roundingQuantity,
			UsageMinimumQuantity: &minimumUsage, UsageMaximumQuantity: &maximumUsage,
			UsageMissingDataPolicy: MissingUsagePendingRecalculation,
		},
		{
			ChargeType: ChargeTypeTrial, PeriodUnit: PeriodUnitDay, PeriodCount: 7,
			PricePhase: PricePhaseNewPurchase, QuantityMode: QuantityModeFixed,
			TrialQuantity: 1, MinimumPurchaseCycles: 1,
		},
	}
	for index, rule := range cases {
		if err := ValidateBillingRule(rule); err != nil {
			t.Fatalf("case=%d rejected a valid billing rule: %v", index, err)
		}
	}
}

func TestValidateBillingRuleRejectsAmbiguousOrUnsafeModels(t *testing.T) {
	t.Parallel()
	cases := []BillingRuleDefinition{
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitYear, 11, PricePhaseBoth),
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitMonth, 121, PricePhaseBoth),
		newFixedBillingRule(ChargeTypeRecurring, PeriodUnitOnce, 1, PricePhaseBoth),
		{
			ChargeType: ChargeTypeOneTime, PeriodUnit: PeriodUnitOnce, PeriodCount: 1,
			PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1,
		},
		newFixedBillingRule(ChargeTypeUsage, PeriodUnitMonth, 1, PricePhaseBoth),
		newFixedBillingRule(ChargeTypeTiered, PeriodUnitMonth, 1, PricePhaseBoth),
	}
	for index, rule := range cases {
		if err := ValidateBillingRule(rule); !bizerr.Is(err, CodeBillingInvalid) {
			t.Fatalf("case=%d returned %v", index, err)
		}
	}
}

func TestValidatePriceScheduleRequiresContinuousGraduatedTiers(t *testing.T) {
	t.Parallel()
	rule := BillingRuleDefinition{
		ChargeType: ChargeTypeTiered, PeriodUnit: PeriodUnitMonth, PeriodCount: 1,
		PricePhase: PricePhaseBoth, QuantityMode: QuantityModeTiered,
		MinimumPurchaseCycles: 1, TierMode: "graduated",
	}
	ten := int64(10)
	twenty := int64(20)
	renewalA := int64(80)
	renewalB := int64(40)
	valid := []PriceTierDefinition{
		{MinimumQuantity: 0, MaximumQuantity: &ten, AmountMinor: 100, SetupFeeMinor: 500, RenewalAmountMinor: &renewalA},
		{MinimumQuantity: 10, MaximumQuantity: &twenty, AmountMinor: 50, RenewalAmountMinor: &renewalB},
		{MinimumQuantity: 20, AmountMinor: 25, RenewalAmountMinor: &renewalB},
	}
	if err := ValidatePriceSchedule(rule, valid); err != nil {
		t.Fatalf("valid graduated schedule rejected: %v", err)
	}
	gap := append([]PriceTierDefinition(nil), valid...)
	gap[1].MinimumQuantity = 11
	if err := ValidatePriceSchedule(rule, gap); !bizerr.Is(err, CodeBillingInvalid) {
		t.Fatalf("gapped schedule returned %v", err)
	}
	overlap := append([]PriceTierDefinition(nil), valid...)
	overlap[1].MinimumQuantity = 9
	if err := ValidatePriceSchedule(rule, overlap); !bizerr.Is(err, CodeBillingInvalid) {
		t.Fatalf("overlapping schedule returned %v", err)
	}
	setupOnLaterTier := append([]PriceTierDefinition(nil), valid...)
	setupOnLaterTier[1].SetupFeeMinor = 1
	if err := ValidatePriceSchedule(rule, setupOnLaterTier); !bizerr.Is(err, CodeBillingInvalid) {
		t.Fatalf("repeated setup fee returned %v", err)
	}
}

func TestQuoteBillingRuleSeparatesSetupAndGraduatedRenewal(t *testing.T) {
	t.Parallel()
	maximumRenewal := 3
	rule := BillingRuleDefinition{
		ChargeType: ChargeTypeTiered, PeriodUnit: PeriodUnitMonth, PeriodCount: 1,
		PricePhase: PricePhaseBoth, QuantityMode: QuantityModeTiered,
		MinimumPurchaseCycles: 2, MaximumRenewalCycles: &maximumRenewal, TierMode: "graduated",
	}
	ten := int64(10)
	twenty := int64(20)
	renewal100 := int64(100)
	renewal50 := int64(50)
	renewal25 := int64(25)
	tiers := []PriceTierDefinition{
		{MinimumQuantity: 0, MaximumQuantity: &ten, AmountMinor: 120, SetupFeeMinor: 500, RenewalAmountMinor: &renewal100},
		{MinimumQuantity: 10, MaximumQuantity: &twenty, AmountMinor: 60, RenewalAmountMinor: &renewal50},
		{MinimumQuantity: 20, AmountMinor: 30, RenewalAmountMinor: &renewal25},
	}
	quantity := int64(25)
	quote, err := QuoteBillingRule(rule, tiers, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: 2, Quantity: &quantity,
	})
	if err != nil {
		t.Fatalf("quote new purchase: %v", err)
	}
	if quote.ChargeAmountMinor == nil || *quote.ChargeAmountMinor != 3_900 ||
		quote.SetupFeeMinor == nil || *quote.SetupFeeMinor != 500 ||
		quote.TotalAmountMinor == nil || *quote.TotalAmountMinor != 4_400 || len(quote.Segments) != 3 {
		t.Fatalf("unexpected new quote: %+v", quote)
	}
	renewal, err := QuoteBillingRule(rule, tiers, BillingQuoteRequest{
		Phase: QuotePhaseRenewal, Cycles: 3, Quantity: &quantity,
	})
	if err != nil {
		t.Fatalf("quote renewal: %v", err)
	}
	if renewal.ChargeAmountMinor == nil || *renewal.ChargeAmountMinor != 4_875 ||
		renewal.SetupFeeMinor == nil || *renewal.SetupFeeMinor != 0 ||
		renewal.TotalAmountMinor == nil || *renewal.TotalAmountMinor != 4_875 {
		t.Fatalf("unexpected renewal quote: %+v", renewal)
	}
	if _, err = QuoteBillingRule(rule, tiers, BillingQuoteRequest{
		Phase: QuotePhaseRenewal, Cycles: 4, Quantity: &quantity,
	}); !bizerr.Is(err, CodeBillingInvalid) {
		t.Fatalf("renewal beyond maximum returned %v", err)
	}
}

func TestQuoteUsageRuleHandlesMissingEstimateRoundingFloorAndCap(t *testing.T) {
	t.Parallel()
	collectionCount := 1
	quantum := int64(10)
	minimum := int64(20)
	maximum := int64(95)
	rule := BillingRuleDefinition{
		ChargeType: ChargeTypeUsage, PeriodUnit: PeriodUnitMonth, PeriodCount: 1,
		PricePhase: PricePhaseBoth, QuantityMode: QuantityModeUsage,
		MinimumPurchaseCycles: 1, MeterUnit: "traffic.gib", TierMode: "graduated",
		UsageCollectionUnit: PeriodUnitHour, UsageCollectionCount: &collectionCount,
		UsageRoundingMode: RoundingModeUp, UsageRoundingQuantity: &quantum,
		UsageMinimumQuantity: &minimum, UsageMaximumQuantity: &maximum,
		UsageMissingDataPolicy: MissingUsageEstimatePrevious,
	}
	fifty := int64(50)
	renewal := int64(1)
	tiers := []PriceTierDefinition{
		{MinimumQuantity: 0, MaximumQuantity: &fifty, AmountMinor: 2, RenewalAmountMinor: &renewal},
		{MinimumQuantity: 50, AmountMinor: 1, RenewalAmountMinor: &renewal},
	}
	estimate := int64(94)
	quote, err := QuoteBillingRule(rule, tiers, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: 1, PreviousCompletePeriodQuantity: &estimate,
	})
	if err != nil {
		t.Fatalf("quote estimated usage: %v", err)
	}
	if quote.Status != BillingQuoteEstimated || quote.BilledQuantity == nil ||
		*quote.BilledQuantity != 95 || quote.TotalAmountMinor == nil || *quote.TotalAmountMinor != 145 {
		t.Fatalf("unexpected estimated quote: %+v", quote)
	}
	rule.UsageMissingDataPolicy = MissingUsagePendingRecalculation
	pending, err := QuoteBillingRule(rule, tiers, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: 1,
	})
	if err != nil || pending.Status != BillingQuotePendingRecalculation || pending.BilledQuantity != nil ||
		pending.ChargeAmountMinor != nil || pending.SetupFeeMinor != nil || pending.TotalAmountMinor != nil {
		t.Fatalf("unexpected pending quote: quote=%+v err=%v", pending, err)
	}
}

func TestQuoteTrialRuleRejectsRepeatedUseAndReturnsExplicitZero(t *testing.T) {
	t.Parallel()
	rule := BillingRuleDefinition{
		ChargeType: ChargeTypeTrial, PeriodUnit: PeriodUnitDay, PeriodCount: 7,
		PricePhase: PricePhaseNewPurchase, QuantityMode: QuantityModeFixed,
		TrialQuantity: 1, MinimumPurchaseCycles: 1,
	}
	quantity := int64(1)
	priorUses := 0
	quote, err := QuoteBillingRule(rule, []PriceTierDefinition{{}}, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: 1, Quantity: &quantity,
		AuthoritativePriorTrialUses: &priorUses,
	})
	if err != nil || quote.TotalAmountMinor == nil || *quote.TotalAmountMinor != 0 ||
		quote.Status != BillingQuoteFinal {
		t.Fatalf("unexpected eligible trial quote=%+v err=%v", quote, err)
	}
	if _, err = QuoteBillingRule(rule, []PriceTierDefinition{{}}, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: 1, Quantity: &quantity,
		AuthoritativePriorTrialUses: intPointer(1),
	}); !bizerr.Is(err, CodeBillingInvalid) {
		t.Fatalf("repeated trial returned %v", err)
	}
}

func intPointer(value int) *int { return &value }

func TestQuoteBillingRuleRejectsIntegerOverflow(t *testing.T) {
	t.Parallel()
	rule := newFixedBillingRule(ChargeTypeRecurring, PeriodUnitMonth, 1, PricePhaseNewPurchase)
	quantity := int64(1)
	_, err := QuoteBillingRule(rule, []PriceTierDefinition{{
		AmountMinor: math.MaxInt64, SetupFeeMinor: 1,
	}}, BillingQuoteRequest{Phase: QuotePhaseNewPurchase, Cycles: 2, Quantity: &quantity})
	if !bizerr.Is(err, CodeBillingAmountOverflow) {
		t.Fatalf("overflow returned %v", err)
	}
}

func newFixedBillingRule(
	chargeType ChargeType,
	periodUnit PeriodUnit,
	periodCount int,
	phase PricePhase,
) BillingRuleDefinition {
	rule := BillingRuleDefinition{
		ChargeType: chargeType, PeriodUnit: periodUnit, PeriodCount: periodCount,
		PricePhase: phase, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1,
	}
	if chargeType == ChargeTypeOneTime {
		rule.PricePhase = PricePhaseNewPurchase
	}
	return rule
}
