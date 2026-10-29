// This file rejects malformed portable price and lifecycle declarations before
// they reach importer persistence; the catalog owner also validates full rules.

package channelcommercecap

import (
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func (p ImportPriceSchedule) validateImportSchedule() error {
	schedule := &productfulfillmentcap.BillingSchedule{CurrencyCode: p.CurrencyCode, ChargeType: productfulfillmentcap.BillingChargeType(p.ChargeType), PeriodUnit: productfulfillmentcap.BillingPeriodUnit(p.PeriodUnit), PeriodCount: p.PeriodCount, PurchaseCycles: p.MinimumPurchaseCycles}
	if schedule.Validate() != nil || p.TermReference == "" || len(p.TermReference) > 4096 || len(p.Options) > 4096 {
		return gerror.New("import price schedule is invalid")
	}
	if err := validateImportBands(p.Base); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, option := range p.Options {
		key := option.FieldKey + "\x00" + option.Value
		if option.FieldKey == "" || seen[key] {
			return gerror.New("import option price is invalid")
		}
		seen[key] = true
		if err := option.validate(); err != nil {
			return err
		}
	}
	return p.WholeItemRule.Validate()
}

// Validate rejects malformed amounts, cycles and chains beyond the public
// bound before any caller marshals or evaluates this rule. A nil rule is valid.
func (r *ImportWholeItemRule) Validate() error {
	seen := make(map[*ImportWholeItemRule]bool)
	for rule := r; rule != nil; rule = rule.Prior {
		if seen[rule] || len(seen) >= MaximumImportWholeItemRuleDepth {
			return gerror.New("import whole-item rule chain is invalid")
		}
		seen[rule] = true
		switch rule.Kind {
		case "fixed":
			if rule.FixedAmountMinor == nil || *rule.FixedAmountMinor < 0 || rule.DiscountBasisPoints != nil {
				return gerror.New("import fixed price is invalid")
			}
		case "discount":
			if rule.DiscountBasisPoints == nil || *rule.DiscountBasisPoints < 0 || *rule.DiscountBasisPoints > 10000 || rule.FixedAmountMinor != nil {
				return gerror.New("import discount is invalid")
			}
		default:
			return gerror.New("import whole-item rule is invalid")
		}
	}
	return nil
}

func validateImportBands(input []ImportPriceBand) error {
	if len(input) == 0 || len(input) > 4096 {
		return gerror.New("import price bands are unavailable")
	}
	bands := append([]ImportPriceBand(nil), input...)
	sort.Slice(bands, func(i, j int) bool { return bands[i].MinimumQuantity < bands[j].MinimumQuantity })
	if bands[0].MinimumQuantity != 0 {
		return gerror.New("import price bands must start at zero")
	}
	for i, band := range bands {
		if band.MinimumQuantity < 0 || band.AmountMinor < 0 || band.SetupAmountMinor < 0 || band.RenewalAmountMinor != nil && *band.RenewalAmountMinor < 0 || i > 0 && band.SetupAmountMinor != 0 {
			return gerror.New("import price band amount is invalid")
		}
		if i == len(bands)-1 {
			if band.MaximumQuantity != nil {
				return gerror.New("import final price band must be unlimited")
			}
		} else if band.MaximumQuantity == nil || *band.MaximumQuantity <= band.MinimumQuantity || *band.MaximumQuantity != bands[i+1].MinimumQuantity {
			return gerror.New("import price bands overlap or have a gap")
		}
	}
	return nil
}

func (f ImportFulfillment) validate() error {
	if f.MaximumPurchaseCycles < 0 || f.MaximumPurchaseCycles > 120 {
		return gerror.New("import maximum purchase cycles are invalid")
	}
	if f.Granularity != productfulfillmentcap.GranularityPerOrderItem && f.Granularity != productfulfillmentcap.GranularityPerQuantityUnit {
		return gerror.New("import fulfillment granularity is invalid")
	}
	switch f.RenewalMode {
	case productfulfillmentcap.RenewalModeUnavailable:
		return nil
	case productfulfillmentcap.RenewalModeLocalEntitlement, productfulfillmentcap.RenewalModeProviderConfirmed:
		if f.OngoingService {
			return nil
		}
	}
	return gerror.New("import fulfillment renewal mode is invalid")
}

// validate keeps quantity algorithms separate from fixed option setup rules.
func (p ImportOptionPrice) validate() error {
	quantity := p.PricingMode == ImportConfigurationPricePerUnit || p.PricingMode == ImportConfigurationPriceBandTotal || p.PricingMode == ImportConfigurationPriceGraduated
	if quantity && p.Value != "" || !quantity && (p.Value == "" || p.PricingMode != "" && p.PricingMode != ImportConfigurationPriceFixed) {
		return gerror.New("invalid import configuration pricing mode")
	}
	bands := append([]ImportPriceBand(nil), p.Bands...)
	if quantity && p.PricingMode != ImportConfigurationPriceGraduated {
		for index := range bands {
			if bands[index].SetupAmountMinor < 0 {
				return gerror.New("import price band amount is invalid")
			}
			bands[index].SetupAmountMinor = 0
		}
	}
	if err := validateImportBands(bands); err != nil {
		return err
	}
	return nil
}
