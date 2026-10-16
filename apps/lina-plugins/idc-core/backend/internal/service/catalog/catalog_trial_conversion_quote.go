// This file prices conversion of an already provisioned trial at the current
// new-purchase rate. Setup fees are removed from every charge, including
// configuration dimensions; no inventory is consumed for the existing service.

package catalog

import "lina-plugin-idc-core/backend/internal/model/entity"

func trialConversionQuoteCharges(line RenewalQuoteLineInput, rule entity.CatalogBillingRule, prices map[string][]entity.CatalogPrice) ([]PurchaseQuoteCharge, error) {
	// Waive setup before calculation, so a large installation fee cannot make
	// an otherwise valid formal period price overflow. Preserve source rows.
	keys := make([]string, 0, len(line.PriceSelections)+1)
	keys = append(keys, purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, ""))
	for _, selection := range line.PriceSelections {
		keys = append(keys, purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, selection.DimensionNo))
	}
	withoutSetup := make(map[string][]entity.CatalogPrice, len(keys))
	for _, key := range keys {
		rows := prices[key]
		copied := append([]entity.CatalogPrice(nil), rows...)
		for index := range copied {
			copied[index].SetupFeeMinor = 0
		}
		withoutSetup[key] = copied
	}
	charges, err := purchaseQuoteCharges(PurchaseQuoteLineInput{
		ReferenceNo: line.ReferenceNo, ProductNo: line.ProductNo, VersionNo: line.VersionNo,
		BillingRuleNo: line.BillingRuleNo, Quantity: line.Quantity, PurchaseCycles: line.PurchaseCycles,
		PriceSelections: line.PriceSelections,
	}, rule, withoutSetup, nil)
	if err != nil {
		return nil, err
	}
	for index := range charges {
		charges[index].SetupFeeMinor = 0
		charges[index].PricedAmountMinor = charges[index].ChargeAmountMinor
	}
	return charges, nil
}
