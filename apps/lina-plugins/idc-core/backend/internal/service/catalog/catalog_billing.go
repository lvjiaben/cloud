package catalog

import (
	"math"
	"regexp"
	"sort"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const maximumBillingTiers = 1_024

type ChargeType string

const (
	ChargeTypeOneTime   ChargeType = "one_time"
	ChargeTypeRecurring ChargeType = "recurring"
	ChargeTypeTiered    ChargeType = "tiered"
	ChargeTypeUsage     ChargeType = "usage"
	ChargeTypeTrial     ChargeType = "trial"
)

type PeriodUnit string

const (
	PeriodUnitOnce  PeriodUnit = "once"
	PeriodUnitHour  PeriodUnit = "hour"
	PeriodUnitDay   PeriodUnit = "day"
	PeriodUnitMonth PeriodUnit = "month"
	PeriodUnitYear  PeriodUnit = "year"
)

type PricePhase string

const (
	PricePhaseNewPurchase PricePhase = "new_purchase"
	PricePhaseRenewal     PricePhase = "renewal"
	PricePhaseBoth        PricePhase = "both"
)

type QuotePhase string

const (
	QuotePhaseNewPurchase QuotePhase = "new_purchase"
	QuotePhaseRenewal     QuotePhase = "renewal"
)

type QuantityMode string

const (
	QuantityModeFixed   QuantityMode = "fixed"
	QuantityModePerUnit QuantityMode = "per_unit"
	QuantityModeTiered  QuantityMode = "tiered"
	QuantityModeUsage   QuantityMode = "usage"
)

type RoundingMode string

const (
	RoundingModeDown    RoundingMode = "down"
	RoundingModeUp      RoundingMode = "up"
	RoundingModeNearest RoundingMode = "nearest"
)

type MissingUsagePolicy string

const (
	MissingUsagePendingRecalculation MissingUsagePolicy = "pending_recalculation"
	MissingUsageEstimatePrevious     MissingUsagePolicy = "estimate_previous_period"
)

type BillingRuleDefinition struct {
	ChargeType             ChargeType
	PeriodUnit             PeriodUnit
	PeriodCount            int
	PricePhase             PricePhase
	QuantityMode           QuantityMode
	TrialQuantity          int
	MinimumPurchaseCycles  int
	MaximumRenewalCycles   *int
	MeterUnit              string
	TierMode               string
	UsageCollectionUnit    PeriodUnit
	UsageCollectionCount   *int
	UsageRoundingMode      RoundingMode
	UsageRoundingQuantity  *int64
	UsageMinimumQuantity   *int64
	UsageMaximumQuantity   *int64
	UsageMissingDataPolicy MissingUsagePolicy
}

type PriceTierDefinition struct {
	MinimumQuantity    int64
	MaximumQuantity    *int64
	AmountMinor        int64
	SetupFeeMinor      int64
	RenewalAmountMinor *int64
}

type BillingQuoteRequest struct {
	Phase                          QuotePhase
	Cycles                         int
	Quantity                       *int64
	PreviousCompletePeriodQuantity *int64
	AuthoritativePriorTrialUses    *int
}

type BillingQuoteStatus string

const (
	BillingQuoteFinal                BillingQuoteStatus = "final"
	BillingQuoteEstimated            BillingQuoteStatus = "estimated"
	BillingQuotePendingRecalculation BillingQuoteStatus = "pending_recalculation"
)

type BillingQuoteSegment struct {
	MinimumQuantity int64
	MaximumQuantity *int64
	BilledQuantity  int64
	UnitAmountMinor int64
	AmountMinor     int64
}

type BillingQuote struct {
	Status            BillingQuoteStatus
	InputQuantity     *int64
	BilledQuantity    *int64
	Cycles            int
	ChargeAmountMinor *int64
	SetupFeeMinor     *int64
	TotalAmountMinor  *int64
	Segments          []BillingQuoteSegment
}

var meterUnitPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

func ValidateBillingRule(rule BillingRuleDefinition) error {
	if !validBillingPeriod(rule.PeriodUnit, rule.PeriodCount) ||
		rule.MinimumPurchaseCycles < 1 || rule.MinimumPurchaseCycles > 120 ||
		(rule.MaximumRenewalCycles != nil &&
			(*rule.MaximumRenewalCycles < 1 || *rule.MaximumRenewalCycles > 120)) ||
		!validPricePhase(rule.PricePhase) {
		return billingInvalid()
	}
	if rule.PricePhase == PricePhaseNewPurchase && rule.MaximumRenewalCycles != nil {
		return billingInvalid()
	}
	if !validBillingModel(rule) {
		return billingInvalid()
	}
	return nil
}

func validBillingModel(rule BillingRuleDefinition) bool {
	switch rule.ChargeType {
	case ChargeTypeOneTime:
		return validOneTimeBillingRule(rule)
	case ChargeTypeRecurring:
		return validRecurringBillingRule(rule)
	case ChargeTypeTiered:
		return validTieredBillingRule(rule)
	case ChargeTypeUsage:
		return validUsageBillingRule(rule)
	case ChargeTypeTrial:
		return validTrialBillingRule(rule)
	default:
		return false
	}
}

func validOneTimeBillingRule(rule BillingRuleDefinition) bool {
	return rule.PeriodUnit == PeriodUnitOnce && rule.PricePhase == PricePhaseNewPurchase &&
		rule.QuantityMode == QuantityModeFixed && rule.TrialQuantity == 0 &&
		rule.MinimumPurchaseCycles == 1 && emptyUsagePolicy(rule)
}

func validRecurringBillingRule(rule BillingRuleDefinition) bool {
	return rule.PeriodUnit != PeriodUnitOnce &&
		(rule.QuantityMode == QuantityModeFixed || rule.QuantityMode == QuantityModePerUnit) &&
		rule.TrialQuantity == 0 && emptyUsagePolicy(rule)
}

func validTieredBillingRule(rule BillingRuleDefinition) bool {
	return rule.PeriodUnit != PeriodUnitOnce && rule.QuantityMode == QuantityModeTiered &&
		rule.TrialQuantity == 0 && rule.TierMode == "graduated" && emptyMeterPolicy(rule)
}

func validUsageBillingRule(rule BillingRuleDefinition) bool {
	return rule.PeriodUnit != PeriodUnitOnce && rule.QuantityMode == QuantityModeUsage &&
		rule.TrialQuantity == 0 && rule.TierMode == "graduated" &&
		meterUnitPattern.MatchString(rule.MeterUnit) && validUsagePolicy(rule)
}

func validTrialBillingRule(rule BillingRuleDefinition) bool {
	return rule.PeriodUnit != PeriodUnitOnce && rule.PricePhase == PricePhaseNewPurchase &&
		rule.QuantityMode == QuantityModeFixed && rule.TrialQuantity >= 1 &&
		rule.TrialQuantity <= 100 && rule.MinimumPurchaseCycles == 1 && emptyUsagePolicy(rule)
}

func ValidatePriceSchedule(rule BillingRuleDefinition, tiers []PriceTierDefinition) error {
	if ValidateBillingRule(rule) != nil || len(tiers) == 0 || len(tiers) > maximumBillingTiers {
		return billingInvalid()
	}
	ordered := append([]PriceTierDefinition(nil), tiers...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].MinimumQuantity < ordered[j].MinimumQuantity
	})
	if ordered[0].MinimumQuantity != 0 {
		return billingInvalid()
	}
	for index, tier := range ordered {
		if tier.MinimumQuantity < 0 || tier.AmountMinor < 0 || tier.SetupFeeMinor < 0 ||
			(tier.RenewalAmountMinor != nil && *tier.RenewalAmountMinor < 0) ||
			(tier.MaximumQuantity != nil && *tier.MaximumQuantity <= tier.MinimumQuantity) {
			return billingInvalid()
		}
		if index > 0 && tier.SetupFeeMinor != 0 {
			return billingInvalid()
		}
		if rule.PricePhase == PricePhaseRenewal && tier.SetupFeeMinor != 0 {
			return billingInvalid()
		}
		if index < len(ordered)-1 {
			if tier.MaximumQuantity == nil || *tier.MaximumQuantity != ordered[index+1].MinimumQuantity {
				return billingInvalid()
			}
		} else if tier.MaximumQuantity != nil {
			return billingInvalid()
		}
		if publicationPricePhaseRequiresRenewal(rule.PricePhase) {
			if tier.RenewalAmountMinor == nil {
				return billingInvalid()
			}
		} else if tier.RenewalAmountMinor != nil {
			return billingInvalid()
		}
		if rule.ChargeType == ChargeTypeTrial &&
			(tier.AmountMinor != 0 || tier.SetupFeeMinor != 0 || tier.RenewalAmountMinor != nil) {
			return billingInvalid()
		}
	}
	if rule.QuantityMode != QuantityModeTiered && rule.QuantityMode != QuantityModeUsage {
		if len(ordered) != 1 || ordered[0].MaximumQuantity != nil {
			return billingInvalid()
		}
	}
	return nil
}

func QuoteBillingRule(
	rule BillingRuleDefinition,
	tiers []PriceTierDefinition,
	request BillingQuoteRequest,
) (*BillingQuote, error) {
	if ValidatePriceSchedule(rule, tiers) != nil || !validQuoteCycles(rule, request.Phase, request.Cycles) {
		return nil, billingInvalid()
	}
	if rule.ChargeType == ChargeTypeTrial {
		if request.AuthoritativePriorTrialUses == nil || *request.AuthoritativePriorTrialUses < 0 ||
			*request.AuthoritativePriorTrialUses >= rule.TrialQuantity {
			return nil, billingInvalid()
		}
	} else if request.AuthoritativePriorTrialUses != nil {
		return nil, billingInvalid()
	}
	quantity, status, err := resolveBillingQuantity(rule, request)
	if err != nil {
		return nil, err
	}
	result := &BillingQuote{Status: status, Cycles: request.Cycles}
	if request.Quantity != nil {
		input := *request.Quantity
		result.InputQuantity = &input
	}
	if status == BillingQuotePendingRecalculation {
		return result, nil
	}
	result.BilledQuantity = &quantity
	ordered := append([]PriceTierDefinition(nil), tiers...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].MinimumQuantity < ordered[j].MinimumQuantity
	})
	segments, recurring, err := calculateBillingSegments(rule, ordered, request.Phase, request.Cycles, quantity)
	if err != nil {
		return nil, err
	}
	result.Segments = segments
	result.ChargeAmountMinor = int64Pointer(recurring)
	setupFee := int64(0)
	if request.Phase == QuotePhaseNewPurchase {
		setupFee = ordered[0].SetupFeeMinor
	}
	result.SetupFeeMinor = int64Pointer(setupFee)
	total, err := addBillingAmount(recurring, setupFee)
	if err != nil {
		return nil, err
	}
	result.TotalAmountMinor = int64Pointer(total)
	return result, nil
}

func BillingRuleDefinitionFromEntity(rule entity.CatalogBillingRule) BillingRuleDefinition {
	return BillingRuleDefinition{
		ChargeType: ChargeType(rule.ChargeTypeCode), PeriodUnit: PeriodUnit(rule.PeriodUnitCode),
		PeriodCount: rule.PeriodCount, PricePhase: PricePhase(rule.PricePhaseCode),
		QuantityMode: QuantityMode(rule.QuantityModeCode), TrialQuantity: rule.TrialQuantity,
		MinimumPurchaseCycles: rule.MinimumPurchaseCycles,
		MaximumRenewalCycles:  cloneIntPointer(rule.MaximumRenewalCycles),
		MeterUnit:             dereferenceString(rule.MeterUnitCode), TierMode: dereferenceString(rule.TierModeCode),
		UsageCollectionUnit:    PeriodUnit(dereferenceString(rule.UsageCollectionUnitCode)),
		UsageCollectionCount:   cloneIntPointer(rule.UsageCollectionCount),
		UsageRoundingMode:      RoundingMode(dereferenceString(rule.UsageRoundingModeCode)),
		UsageRoundingQuantity:  cloneInt64Pointer(rule.UsageRoundingQuantity),
		UsageMinimumQuantity:   cloneInt64Pointer(rule.UsageMinimumQuantity),
		UsageMaximumQuantity:   cloneInt64Pointer(rule.UsageMaximumQuantity),
		UsageMissingDataPolicy: MissingUsagePolicy(dereferenceString(rule.UsageMissingPolicyCode)),
	}
}

func PriceTierDefinitionFromEntity(price entity.CatalogPrice) PriceTierDefinition {
	return PriceTierDefinition{
		MinimumQuantity: price.MinimumQuantity, MaximumQuantity: cloneInt64Pointer(price.MaximumQuantity),
		AmountMinor: price.AmountMinor, SetupFeeMinor: price.SetupFeeMinor,
		RenewalAmountMinor: cloneInt64Pointer(price.RenewalAmountMinor),
	}
}

func validBillingPeriod(unit PeriodUnit, count int) bool {
	switch unit {
	case PeriodUnitOnce:
		return count == 1
	case PeriodUnitHour:
		return count >= 1 && count <= 87_600
	case PeriodUnitDay:
		return count >= 1 && count <= 3_650
	case PeriodUnitMonth:
		return count >= 1 && count <= 120
	case PeriodUnitYear:
		return count >= 1 && count <= 10
	default:
		return false
	}
}

func validPricePhase(phase PricePhase) bool {
	return phase == PricePhaseNewPurchase || phase == PricePhaseRenewal || phase == PricePhaseBoth
}

func publicationPricePhaseRequiresRenewal(phase PricePhase) bool {
	return phase == PricePhaseRenewal || phase == PricePhaseBoth
}

func emptyUsagePolicy(rule BillingRuleDefinition) bool {
	return rule.TierMode == "" && emptyMeterPolicy(rule)
}

func emptyMeterPolicy(rule BillingRuleDefinition) bool {
	return rule.MeterUnit == "" && rule.UsageCollectionUnit == "" &&
		rule.UsageCollectionCount == nil && rule.UsageRoundingMode == "" &&
		rule.UsageRoundingQuantity == nil && rule.UsageMinimumQuantity == nil &&
		rule.UsageMaximumQuantity == nil && rule.UsageMissingDataPolicy == ""
}

func validUsagePolicy(rule BillingRuleDefinition) bool {
	if (rule.UsageCollectionUnit != PeriodUnitHour && rule.UsageCollectionUnit != PeriodUnitDay &&
		rule.UsageCollectionUnit != PeriodUnitMonth) || rule.UsageCollectionCount == nil ||
		*rule.UsageCollectionCount < 1 || *rule.UsageCollectionCount > 120 ||
		(rule.UsageRoundingMode != RoundingModeDown && rule.UsageRoundingMode != RoundingModeUp &&
			rule.UsageRoundingMode != RoundingModeNearest) || rule.UsageRoundingQuantity == nil ||
		*rule.UsageRoundingQuantity <= 0 || rule.UsageMinimumQuantity == nil ||
		*rule.UsageMinimumQuantity < 0 ||
		(rule.UsageMaximumQuantity != nil && *rule.UsageMaximumQuantity < *rule.UsageMinimumQuantity) {
		return false
	}
	return rule.UsageMissingDataPolicy == MissingUsagePendingRecalculation ||
		rule.UsageMissingDataPolicy == MissingUsageEstimatePrevious
}

func validQuoteCycles(rule BillingRuleDefinition, phase QuotePhase, cycles int) bool {
	if cycles < 1 || cycles > 120 {
		return false
	}
	if rule.ChargeType == ChargeTypeOneTime || rule.ChargeType == ChargeTypeTrial {
		return phase == QuotePhaseNewPurchase && cycles == 1
	}
	switch phase {
	case QuotePhaseNewPurchase:
		return rule.PricePhase != PricePhaseRenewal && cycles >= rule.MinimumPurchaseCycles
	case QuotePhaseRenewal:
		if rule.PricePhase == PricePhaseNewPurchase {
			return false
		}
		return rule.MaximumRenewalCycles == nil || cycles <= *rule.MaximumRenewalCycles
	default:
		return false
	}
}

func resolveBillingQuantity(
	rule BillingRuleDefinition,
	request BillingQuoteRequest,
) (int64, BillingQuoteStatus, error) {
	if rule.QuantityMode != QuantityModeUsage {
		if request.Quantity == nil || *request.Quantity < 0 || request.PreviousCompletePeriodQuantity != nil {
			return 0, "", billingInvalid()
		}
		if rule.QuantityMode == QuantityModeFixed && *request.Quantity != 1 {
			return 0, "", billingInvalid()
		}
		return *request.Quantity, BillingQuoteFinal, nil
	}
	quantity := request.Quantity
	status := BillingQuoteFinal
	if quantity == nil {
		if rule.UsageMissingDataPolicy == MissingUsagePendingRecalculation {
			if request.PreviousCompletePeriodQuantity != nil {
				return 0, "", billingInvalid()
			}
			return 0, BillingQuotePendingRecalculation, nil
		}
		if request.PreviousCompletePeriodQuantity == nil {
			return 0, BillingQuotePendingRecalculation, nil
		}
		quantity = request.PreviousCompletePeriodQuantity
		status = BillingQuoteEstimated
	} else if request.PreviousCompletePeriodQuantity != nil {
		return 0, "", billingInvalid()
	}
	if *quantity < 0 {
		return 0, "", billingInvalid()
	}
	normalized, err := normalizeUsageQuantity(rule, *quantity)
	return normalized, status, err
}

func normalizeUsageQuantity(rule BillingRuleDefinition, quantity int64) (int64, error) {
	quantum := *rule.UsageRoundingQuantity
	remainder := quantity % quantum
	if remainder != 0 {
		switch rule.UsageRoundingMode {
		case RoundingModeDown:
			quantity -= remainder
		case RoundingModeUp:
			var err error
			quantity, err = addBillingAmount(quantity, quantum-remainder)
			if err != nil {
				return 0, err
			}
		case RoundingModeNearest:
			if remainder < quantum-remainder {
				quantity -= remainder
			} else {
				var err error
				quantity, err = addBillingAmount(quantity, quantum-remainder)
				if err != nil {
					return 0, err
				}
			}
		default:
			return 0, billingInvalid()
		}
	}
	if quantity < *rule.UsageMinimumQuantity {
		quantity = *rule.UsageMinimumQuantity
	}
	if rule.UsageMaximumQuantity != nil && quantity > *rule.UsageMaximumQuantity {
		quantity = *rule.UsageMaximumQuantity
	}
	return quantity, nil
}

func calculateBillingSegments(
	rule BillingRuleDefinition,
	tiers []PriceTierDefinition,
	phase QuotePhase,
	cycles int,
	quantity int64,
) ([]BillingQuoteSegment, int64, error) {
	if rule.ChargeType == ChargeTypeTrial {
		return []BillingQuoteSegment{}, 0, nil
	}
	if rule.QuantityMode == QuantityModeFixed {
		amount, err := tierUnitAmount(tiers[0], phase)
		if err != nil {
			return nil, 0, err
		}
		return singleBillingSegment(tiers[0], 1, amount, billingCycleMultiplier(rule, cycles))
	}
	if rule.QuantityMode == QuantityModePerUnit {
		amount, err := tierUnitAmount(tiers[0], phase)
		if err != nil {
			return nil, 0, err
		}
		return singleBillingSegment(tiers[0], quantity, amount, billingCycleMultiplier(rule, cycles))
	}
	segments := make([]BillingQuoteSegment, 0, len(tiers))
	total := int64(0)
	for _, tier := range tiers {
		if quantity <= tier.MinimumQuantity {
			break
		}
		upper := quantity
		if tier.MaximumQuantity != nil && upper > *tier.MaximumQuantity {
			upper = *tier.MaximumQuantity
		}
		billed := upper - tier.MinimumQuantity
		unit, err := tierUnitAmount(tier, phase)
		if err != nil {
			return nil, 0, err
		}
		amount, err := multiplyBillingAmount(unit, billed)
		if err == nil {
			amount, err = multiplyBillingAmount(amount, int64(billingCycleMultiplier(rule, cycles)))
		}
		if err != nil {
			return nil, 0, err
		}
		total, err = addBillingAmount(total, amount)
		if err != nil {
			return nil, 0, err
		}
		maximum := tier.MaximumQuantity
		segments = append(segments, BillingQuoteSegment{
			MinimumQuantity: tier.MinimumQuantity, MaximumQuantity: maximum,
			BilledQuantity: billed, UnitAmountMinor: unit, AmountMinor: amount,
		})
	}
	return segments, total, nil
}

func singleBillingSegment(
	tier PriceTierDefinition,
	quantity int64,
	unitAmount int64,
	cycles int,
) ([]BillingQuoteSegment, int64, error) {
	amount, err := multiplyBillingAmount(unitAmount, quantity)
	if err == nil {
		amount, err = multiplyBillingAmount(amount, int64(cycles))
	}
	if err != nil {
		return nil, 0, err
	}
	return []BillingQuoteSegment{{
		MinimumQuantity: 0, MaximumQuantity: nil, BilledQuantity: quantity,
		UnitAmountMinor: unitAmount, AmountMinor: amount,
	}}, amount, nil
}

func billingCycleMultiplier(rule BillingRuleDefinition, cycles int) int {
	if rule.ChargeType == ChargeTypeOneTime || rule.ChargeType == ChargeTypeTrial {
		return 1
	}
	return cycles
}

func tierUnitAmount(tier PriceTierDefinition, phase QuotePhase) (int64, error) {
	if phase == QuotePhaseNewPurchase {
		return tier.AmountMinor, nil
	}
	if tier.RenewalAmountMinor == nil {
		return 0, billingInvalid()
	}
	return *tier.RenewalAmountMinor, nil
}

func multiplyBillingAmount(left, right int64) (int64, error) {
	if left < 0 || right < 0 || (left != 0 && right > math.MaxInt64/left) {
		return 0, bizerr.NewCode(CodeBillingAmountOverflow)
	}
	return left * right, nil
}

func addBillingAmount(left, right int64) (int64, error) {
	if left < 0 || right < 0 || right > math.MaxInt64-left {
		return 0, bizerr.NewCode(CodeBillingAmountOverflow)
	}
	return left + right, nil
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64Pointer(value int64) *int64 {
	return &value
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func billingInvalid() error {
	return bizerr.NewCode(CodeBillingInvalid)
}
