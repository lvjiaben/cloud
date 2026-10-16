// This file prices one frozen configuration independently from the product's
// base quantity model. Bands use (minimum, maximum] quantity boundaries; zero
// selection waives both the configuration charge and its installation fee.

package catalog

import (
	"sort"
)

// ConfigurationPriceMode fixes the meaning of a configuration price schedule.
// Existing selectable options use fixed; quantity fields explicitly choose
// per_unit or band_total before their immutable dimension is created.
type ConfigurationPriceMode string

const (
	ConfigurationPriceFixed     ConfigurationPriceMode = "fixed"
	ConfigurationPricePerUnit   ConfigurationPriceMode = "per_unit"
	ConfigurationPriceBandTotal ConfigurationPriceMode = "band_total"
)

// validateConfigurationPriceSchedule preserves the product's cycle and renewal
// rules while permitting independent quantity bands for a configuration.
func validateConfigurationPriceSchedule(rule BillingRuleDefinition, mode ConfigurationPriceMode, tiers []PriceTierDefinition) error {
	if mode == "" || mode == ConfigurationPriceFixed {
		return ValidatePriceSchedule(rule, tiers)
	}
	if mode != ConfigurationPricePerUnit && mode != ConfigurationPriceBandTotal {
		return billingInvalid()
	}
	if len(tiers) == 0 || len(tiers) > maximumBillingTiers {
		return billingInvalid()
	}
	ordered := append([]PriceTierDefinition(nil), tiers...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].MinimumQuantity < ordered[j].MinimumQuantity })
	if ordered[0].MinimumQuantity != 0 {
		return billingInvalid()
	}
	for index, tier := range ordered {
		if tier.MinimumQuantity < 0 || (tier.MaximumQuantity != nil && *tier.MaximumQuantity <= tier.MinimumQuantity) {
			return billingInvalid()
		}
		if index < len(ordered)-1 {
			if tier.MaximumQuantity == nil || *tier.MaximumQuantity != ordered[index+1].MinimumQuantity {
				return billingInvalid()
			}
		} else if tier.MaximumQuantity != nil {
			return billingInvalid()
		}
		// Validate cycle, money, trial and renewal semantics with the existing owner
		// calculator, without inheriting its unrelated base quantity model.
		normalized := tier
		normalized.MinimumQuantity, normalized.MaximumQuantity = 0, nil
		fixed := rule
		fixed.QuantityMode, fixed.TierMode = QuantityModeFixed, ""
		if err := ValidatePriceSchedule(fixed, []PriceTierDefinition{normalized}); err != nil {
			return err
		}
	}
	return nil
}

// quoteConfigurationPrice uses the selected band's unit or total amount for
// both purchases and renewals. Overflow and invalid schedules fail closed.
func quoteConfigurationPrice(rule BillingRuleDefinition, mode ConfigurationPriceMode, tiers []PriceTierDefinition, request BillingQuoteRequest) (*BillingQuote, error) {
	if mode == "" || mode == ConfigurationPriceFixed {
		return QuoteBillingRule(rule, tiers, request)
	}
	if err := validateConfigurationPriceSchedule(rule, mode, tiers); err != nil {
		return nil, err
	}
	if request.Quantity == nil || *request.Quantity < 0 {
		return nil, billingInvalid()
	}
	quantity := *request.Quantity
	selected := tiers[0]
	for _, tier := range tiers {
		if quantity > tier.MinimumQuantity && (tier.MaximumQuantity == nil || quantity <= *tier.MaximumQuantity) {
			selected = tier
			break
		}
	}
	selected.MinimumQuantity, selected.MaximumQuantity = 0, nil
	definition := rule
	definition.QuantityMode, definition.TierMode = QuantityModeFixed, ""
	one := int64(1)
	fixedRequest := request
	fixedRequest.Quantity = &one
	result, err := QuoteBillingRule(definition, []PriceTierDefinition{selected}, fixedRequest)
	if err != nil {
		return nil, err
	}
	result.InputQuantity, result.BilledQuantity = int64Pointer(quantity), int64Pointer(quantity)
	if quantity == 0 {
		result.ChargeAmountMinor, result.SetupFeeMinor, result.TotalAmountMinor = int64Pointer(0), int64Pointer(0), int64Pointer(0)
		result.Segments = []BillingQuoteSegment{}
		return result, nil
	}
	if mode == ConfigurationPricePerUnit {
		amount, multiplyErr := multiplyBillingAmount(*result.ChargeAmountMinor, quantity)
		if multiplyErr != nil {
			return nil, multiplyErr
		}
		total, addErr := addBillingAmount(amount, *result.SetupFeeMinor)
		if addErr != nil {
			return nil, addErr
		}
		result.ChargeAmountMinor, result.TotalAmountMinor = int64Pointer(amount), int64Pointer(total)
	}
	if len(result.Segments) == 1 {
		result.Segments[0].MinimumQuantity = 0
		result.Segments[0].BilledQuantity = quantity
		result.Segments[0].AmountMinor = *result.ChargeAmountMinor
	}
	return result, nil
}
