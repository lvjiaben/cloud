package commerce

import (
	"math/big"
	"sort"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

type quoteNetComponent struct {
	amount    int64
	setup     bool
	lineIndex int
}

type quoteItemAmounts struct {
	setupSubtotal  int64
	periodSubtotal int64
	subtotal       int64
	discount       int64
	net            int64
	tax            int64
	total          int64
}

// PricingAmounts is the immutable tax-exclusive amount composition shared by
// new-purchase and renewal quote owners.
type PricingAmounts struct {
	SetupSubtotalMinor  int64
	PeriodSubtotalMinor int64
	SubtotalMinor       int64
	DiscountMinor       int64
	NetAmountMinor      int64
	TaxAmountMinor      int64
	TotalAmountMinor    int64
}

// PricingChargeAmount is the tax-exclusive amount attributable to one catalog
// charge before quote-wide discounts.
type PricingChargeAmount struct {
	ChargeIndex         int
	SetupAmountMinor    int64
	PeriodicAmountMinor int64
}

// CalculatePricingAmounts applies the one authoritative commerce tax and
// rounding algorithm to a catalog-produced quote line.
func CalculatePricingAmounts(
	line catalog.PurchaseQuoteLine,
	discountMinor int64,
) (PricingAmounts, error) {
	amounts, err := calculateQuoteItemAmounts(line, discountMinor)
	if err != nil {
		return PricingAmounts{}, err
	}
	return PricingAmounts{
		SetupSubtotalMinor: amounts.setupSubtotal, PeriodSubtotalMinor: amounts.periodSubtotal,
		SubtotalMinor: amounts.subtotal, DiscountMinor: amounts.discount,
		NetAmountMinor: amounts.net, TaxAmountMinor: amounts.tax, TotalAmountMinor: amounts.total,
	}, nil
}

// CalculatePricingChargeAmounts exposes the same tax extraction used by quote
// totals while preserving each catalog charge's stable input index.
func CalculatePricingChargeAmounts(
	line catalog.PurchaseQuoteLine,
) ([]PricingChargeAmount, error) {
	components, err := quoteTaxExclusiveComponents(line)
	if err != nil {
		return nil, err
	}
	result := make([]PricingChargeAmount, len(line.Charges))
	for index := range result {
		result[index].ChargeIndex = index
	}
	for _, component := range components {
		if component.lineIndex < 0 || component.lineIndex >= len(result) {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		if component.setup {
			result[component.lineIndex].SetupAmountMinor, err = safeQuoteAmountAdd(
				result[component.lineIndex].SetupAmountMinor, component.amount,
			)
		} else {
			result[component.lineIndex].PeriodicAmountMinor, err = safeQuoteAmountAdd(
				result[component.lineIndex].PeriodicAmountMinor, component.amount,
			)
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func calculateQuoteItemAmounts(
	line catalog.PurchaseQuoteLine,
	discount int64,
) (quoteItemAmounts, error) {
	components, err := quoteTaxExclusiveComponents(line)
	if err != nil {
		return quoteItemAmounts{}, err
	}
	amounts := quoteItemAmounts{discount: discount}
	weights := make([]int64, len(components))
	for index, component := range components {
		weights[index] = component.amount
		if component.setup {
			amounts.setupSubtotal, err = safeQuoteAmountAdd(amounts.setupSubtotal, component.amount)
		} else {
			amounts.periodSubtotal, err = safeQuoteAmountAdd(amounts.periodSubtotal, component.amount)
		}
		if err != nil {
			return quoteItemAmounts{}, err
		}
	}
	amounts.subtotal, err = safeQuoteAmountAdd(amounts.setupSubtotal, amounts.periodSubtotal)
	if err != nil || discount < 0 || discount > amounts.subtotal {
		return quoteItemAmounts{}, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	amounts.net = amounts.subtotal - discount
	if line.TaxTreatmentCode == string(catalog.TaxTreatmentTaxable) {
		if line.TaxRateBasisPoints == nil {
			return quoteItemAmounts{}, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		rate := int64(*line.TaxRateBasisPoints)
		if line.RoundingScopeCode == string(catalog.TaxRoundingScopeLine) {
			allocated, allocateErr := allocateQuoteAmount(discount, weights)
			if allocateErr != nil {
				return quoteItemAmounts{}, allocateErr
			}
			taxableLines := make([]int64, len(line.Charges))
			for index, component := range components {
				taxableLines[component.lineIndex], err = safeQuoteAmountAdd(
					taxableLines[component.lineIndex], component.amount-allocated[index],
				)
				if err != nil {
					return quoteItemAmounts{}, err
				}
			}
			for _, taxableAmount := range taxableLines {
				lineTax, taxErr := catalog.RoundTaxRational(
					taxableAmount, rate, 10_000,
					line.RoundingQuantumMinor, catalog.TaxRoundingMode(line.RoundingModeCode),
				)
				if taxErr != nil {
					return quoteItemAmounts{}, bizerr.NewCode(CodeQuoteNotAvailable)
				}
				amounts.tax, err = safeQuoteAmountAdd(amounts.tax, lineTax)
				if err != nil {
					return quoteItemAmounts{}, err
				}
			}
		} else if line.RoundingScopeCode == string(catalog.TaxRoundingScopeTotal) {
			amounts.tax, err = catalog.RoundTaxRational(
				amounts.net, rate, 10_000, line.RoundingQuantumMinor,
				catalog.TaxRoundingMode(line.RoundingModeCode),
			)
			if err != nil {
				return quoteItemAmounts{}, bizerr.NewCode(CodeQuoteNotAvailable)
			}
		} else {
			return quoteItemAmounts{}, bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	amounts.total, err = safeQuoteAmountAdd(amounts.net, amounts.tax)
	if err != nil {
		return quoteItemAmounts{}, err
	}
	return amounts, nil
}

func quoteTaxExclusiveComponents(line catalog.PurchaseQuoteLine) ([]quoteNetComponent, error) {
	components := make([]quoteNetComponent, 0, len(line.Charges)*2)
	for lineIndex, charge := range line.Charges {
		if charge.SetupFeeMinor > 0 {
			components = append(components, quoteNetComponent{
				amount: charge.SetupFeeMinor, setup: true, lineIndex: lineIndex,
			})
		}
		if charge.ChargeAmountMinor > 0 {
			components = append(components, quoteNetComponent{
				amount:    charge.ChargeAmountMinor,
				setup:     line.PeriodUnitCode == string(catalog.PeriodUnitOnce),
				lineIndex: lineIndex,
			})
		}
	}
	if line.TaxTreatmentCode != string(catalog.TaxTreatmentTaxable) || !line.PriceIncludesTax || len(components) == 0 {
		return components, nil
	}
	if line.TaxRateBasisPoints == nil {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	rate := int64(*line.TaxRateBasisPoints)
	weights := make([]int64, len(components))
	pricedTotal := int64(0)
	for index, component := range components {
		weights[index] = component.amount
		var err error
		pricedTotal, err = safeQuoteAmountAdd(pricedTotal, component.amount)
		if err != nil {
			return nil, err
		}
	}
	embeddedByComponent := make([]int64, len(components))
	if line.RoundingScopeCode == string(catalog.TaxRoundingScopeLine) {
		lineWeights := make([][]int64, len(line.Charges))
		lineIndexes := make([][]int, len(line.Charges))
		lineTotals := make([]int64, len(line.Charges))
		var err error
		for index, component := range components {
			lineWeights[component.lineIndex] = append(lineWeights[component.lineIndex], component.amount)
			lineIndexes[component.lineIndex] = append(lineIndexes[component.lineIndex], index)
			lineTotals[component.lineIndex], err = safeQuoteAmountAdd(
				lineTotals[component.lineIndex], component.amount,
			)
			if err != nil {
				return nil, err
			}
		}
		for lineIndex, lineTotal := range lineTotals {
			tax, err := catalog.RoundTaxRational(
				lineTotal, rate, 10_000+rate, line.RoundingQuantumMinor,
				catalog.TaxRoundingMode(line.RoundingModeCode),
			)
			if err != nil || tax > lineTotal {
				return nil, bizerr.NewCode(CodeQuoteNotAvailable)
			}
			allocated, allocateErr := allocateQuoteAmount(tax, lineWeights[lineIndex])
			if allocateErr != nil {
				return nil, allocateErr
			}
			for offset, componentIndex := range lineIndexes[lineIndex] {
				embeddedByComponent[componentIndex] = allocated[offset]
			}
		}
	} else if line.RoundingScopeCode == string(catalog.TaxRoundingScopeTotal) {
		tax, err := catalog.RoundTaxRational(
			pricedTotal, rate, 10_000+rate, line.RoundingQuantumMinor,
			catalog.TaxRoundingMode(line.RoundingModeCode),
		)
		if err != nil || tax > pricedTotal {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		embeddedByComponent, err = allocateQuoteAmount(tax, weights)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	for index := range components {
		components[index].amount -= embeddedByComponent[index]
	}
	return components, nil
}

type quoteAllocationRemainder struct {
	index     int
	remainder *big.Int
}

// allocateQuoteAmount uses largest remainders with stable input-order ties.
func allocateQuoteAmount(total int64, weights []int64) ([]int64, error) {
	result := make([]int64, len(weights))
	weightTotal := int64(0)
	var err error
	for _, weight := range weights {
		if weight < 0 {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		weightTotal, err = safeQuoteAmountAdd(weightTotal, weight)
		if err != nil {
			return nil, err
		}
	}
	if total < 0 || total > weightTotal || (weightTotal == 0 && total != 0) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	if total == 0 {
		return result, nil
	}
	denominator := big.NewInt(weightTotal)
	allocated := int64(0)
	remainders := make([]quoteAllocationRemainder, 0, len(weights))
	for index, weight := range weights {
		numerator := new(big.Int).Mul(big.NewInt(total), big.NewInt(weight))
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(numerator, denominator, remainder)
		if !quotient.IsInt64() {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		result[index] = quotient.Int64()
		allocated += result[index]
		remainders = append(remainders, quoteAllocationRemainder{index: index, remainder: remainder})
	}
	sort.SliceStable(remainders, func(left, right int) bool {
		return remainders[left].remainder.Cmp(remainders[right].remainder) > 0
	})
	for remainder := total - allocated; remainder > 0; remainder-- {
		result[remainders[0].index]++
		remainders = remainders[1:]
	}
	return result, nil
}

func safeQuoteAmountAdd(left, right int64) (int64, error) {
	if left < 0 || right < 0 || left > int64(^uint64(0)>>1)-right {
		return 0, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return left + right, nil
}
