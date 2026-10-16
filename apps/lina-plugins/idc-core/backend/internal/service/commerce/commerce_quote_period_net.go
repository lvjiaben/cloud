// This file exposes the paid periodic portion needed by configuration
// upgrades, using the same tax extraction and discount allocation as checkout.

package commerce

import "lina-plugin-idc-core/backend/internal/service/catalog"

// CalculatePeriodicNetAmountWithAdjustments returns the product/configuration
// periodic charge after discounts and before tax, excluding installation.
// The caller must supply the immutable paid quote, never current catalog
// prices. Ordinary discounts allocate over setup and periodic components;
// negotiated agent prices retain the checkout installation exclusion.
func CalculatePeriodicNetAmountWithAdjustments(line catalog.PurchaseQuoteLine, adjustments []QuoteAdjustment) (int64, error) {
	discount, agent, err := quoteDiscountScope(adjustments)
	if err != nil {
		return 0, err
	}
	components, err := quoteTaxExclusiveComponents(line)
	if err != nil {
		return 0, err
	}
	weights := make([]int64, len(components))
	for index, component := range components {
		if !agent || !component.installation {
			weights[index] = component.amount
		}
	}
	allocated, err := allocateQuoteAmount(discount, weights)
	if err != nil {
		return 0, err
	}
	var result int64
	for index, component := range components {
		if component.setup {
			continue
		}
		result, err = safeQuoteAmountAdd(result, component.amount-allocated[index])
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}
