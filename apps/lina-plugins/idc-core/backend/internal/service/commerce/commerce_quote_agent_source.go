// This file replaces catalog periodic amounts with an immutable negotiated
// price source. Setup amounts retain their original tax-exclusive value.
package commerce

import (
	"math/big"
	"sort"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// ApplyAgentPrice freezes a negotiated price as the charge source, rather than
// a negative marketing discount. The same function is used for renewals.
func ApplyAgentPrice(line catalog.PurchaseQuoteLine, adjustments []QuoteAdjustment) (catalog.PurchaseQuoteLine, error) {
	var source *QuoteAdjustment
	for index := range adjustments {
		adjustment := &adjustments[index]
		if adjustment.TypeCode != QuoteAdjustmentAgentPrice || adjustment.TargetPeriodAmountMinor == nil {
			continue
		}
		if source != nil || len(adjustments) != 1 || adjustment.AmountMinor != 0 || adjustment.OriginalPeriodAmountMinor == nil || *adjustment.TargetPeriodAmountMinor < 0 {
			return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		source = adjustment
	}
	if source == nil {
		return line, nil
	}
	basis, err := agentPriceBasis(line)
	if err != nil || basis != *source.OriginalPeriodAmountMinor {
		return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	components, err := quoteTaxExclusiveComponents(line)
	if err != nil {
		return catalog.PurchaseQuoteLine{}, err
	}
	weights := make([]int64, len(line.Charges))
	setups := make([]int64, len(line.Charges))
	for _, component := range components {
		if component.installation {
			setups[component.lineIndex] = component.amount
		} else {
			weights[component.lineIndex] = component.amount
		}
	}
	amounts, err := allocateAgentPrice(*source.TargetPeriodAmountMinor, weights)
	if err != nil {
		return catalog.PurchaseQuoteLine{}, err
	}
	line.Charges = append([]catalog.PurchaseQuoteCharge(nil), line.Charges...)
	for index := range line.Charges {
		charge := &line.Charges[index]
		charge.ChargeAmountMinor = amounts[index]
		charge.SetupFeeMinor = setups[index]
		charge.PricedAmountMinor, err = safeQuoteAmountAdd(amounts[index], setups[index])
		if err != nil {
			return catalog.PurchaseQuoteLine{}, err
		}
		charge.PriceSourceDigest, err = quoteContentDigest(struct {
			CatalogDigest   string
			AgentSource     QuoteAdjustment
			Periodic, Setup int64
		}{charge.PriceSourceDigest, *source, amounts[index], setups[index]})
		if err != nil {
			return catalog.PurchaseQuoteLine{}, err
		}
	}
	// All components now hold their tax-exclusive amount. Tax policy and
	// rounding remain unchanged; embedded tax must not be extracted twice.
	line.PriceIncludesTax = false
	return line, nil
}

// allocateAgentPrice distributes any nonnegative target (including a markup)
// over existing configuration charges with deterministic largest remainders.
func allocateAgentPrice(target int64, weights []int64) ([]int64, error) {
	if target < 0 || len(weights) == 0 {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	var total int64
	for _, weight := range weights {
		var err error
		total, err = safeQuoteAmountAdd(total, weight)
		if err != nil {
			return nil, err
		}
	}
	result := make([]int64, len(weights))
	if total == 0 {
		result[0] = target
		return result, nil
	}
	var allocated int64
	remainders := make([]quoteAllocationRemainder, len(weights))
	for index, weight := range weights {
		numerator := new(big.Int).Mul(big.NewInt(target), big.NewInt(weight))
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(numerator, big.NewInt(total), remainder)
		result[index] = quotient.Int64()
		allocated += result[index]
		remainders[index] = quoteAllocationRemainder{index: index, remainder: remainder}
	}
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].remainder.Cmp(remainders[j].remainder) > 0 })
	for index := int64(0); index < target-allocated; index++ {
		result[remainders[index].index]++
	}
	return result, nil
}
