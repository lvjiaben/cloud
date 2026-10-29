// This file saves one ordinary billing period and its first complete base price
// under the existing publication transaction, without a partially priced period.
package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
)

// PricedBillingRuleCreateInput describes one fixed-price purchase period. A
// recurring period requires an explicit renewal price; no money is inferred.
type PricedBillingRuleCreateInput struct {
	VersionNo             string
	RuleCode              string
	PeriodUnit            PeriodUnit
	PeriodCount           int
	MinimumPurchaseCycles int
	MaximumRenewalCycles  *int
	CurrencyCode          string
	Price                 PriceTierDefinition
}

// CreatePricedBillingRule applies the billing rule and its first base price as
// one edit. Both action permissions and audits remain with their owners. Failed
// validation, stale state or audit rolls back the rule, price and application.
func CreatePricedBillingRule(ctx context.Context, manager ManagementService, publication PublicationService, input PricedBillingRuleCreateInput) (*BillingRule, error) {
	if input.Price.MinimumQuantity != 0 || input.Price.MaximumQuantity != nil {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	definition := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: input.PeriodUnit, PeriodCount: input.PeriodCount, PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: input.MinimumPurchaseCycles, MaximumRenewalCycles: input.MaximumRenewalCycles}
	if input.PeriodUnit == PeriodUnitOnce {
		definition.ChargeType = ChargeTypeOneTime
		definition.PricePhase = PricePhaseNewPurchase
	}
	var result *BillingRule
	err := publication.WithSavedSettings(ctx, input.VersionNo, func(ctx context.Context) error {
		created, err := manager.CreateBillingRule(ctx, BillingRuleCreateInput{VersionNo: input.VersionNo, RuleCode: input.RuleCode, Definition: definition, Active: true})
		if err != nil {
			return err
		}
		err = manager.SaveConfigurationPriceBatch(ctx, input.VersionNo, []ConfigurationPriceBatchRow{{TargetKind: PriceBatchTargetBase, Schedule: PriceScheduleCreateInput{VersionNo: input.VersionNo, BillingRuleNo: created.BillingRuleNo, CurrencyCode: input.CurrencyCode, Tiers: []PriceTierDefinition{input.Price}, Active: true}}}, nil)
		if err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
