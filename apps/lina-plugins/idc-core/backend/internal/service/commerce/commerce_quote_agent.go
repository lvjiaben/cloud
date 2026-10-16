// This file composes agent pricing and marketing through the same evaluator
// for customer checkout and authenticated channel checkout.
package commerce

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

type agentQuoteAdjustmentEvaluator struct {
	pricing   salesmanagement.PricingService
	marketing QuoteAdjustmentEvaluator
}

// NewAgentQuoteAdjustmentEvaluator excludes matched agent-price lines from
// marketing, preserves immutable source digests, and reuses reservation owners.
func NewAgentQuoteAdjustmentEvaluator(pricing salesmanagement.PricingService, marketing QuoteAdjustmentEvaluator) (QuoteAdjustmentEvaluator, error) {
	if pricing == nil || marketing == nil {
		return nil, gerror.New("agent quote evaluator requires pricing and marketing owners")
	}
	return &agentQuoteAdjustmentEvaluator{pricing: pricing, marketing: marketing}, nil
}
func (a *agentQuoteAdjustmentEvaluator) EvaluateAndReserveQuote(ctx context.Context, input QuoteAdjustmentInput) (*QuoteAdjustmentResult, error) {
	request := salesmanagement.PriceEvaluationInput{CustomerNo: input.CustomerNo, CurrencyCode: input.CurrencyCode, Now: input.IssuedAt, Lines: make([]salesmanagement.PriceLine, 0, len(input.Lines))}
	for _, line := range input.Lines {
		request.Lines = append(request.Lines, salesmanagement.PriceLine{ReferenceNo: line.ReferenceNo, ProductNo: line.ProductNo, ChargeTypeCode: line.ChargeTypeCode, PeriodUnitCode: line.PeriodUnitCode, PeriodCount: line.PeriodCount, Quantity: line.Quantity, PurchaseCycles: line.PurchaseCycles, PeriodSubtotalMinor: line.PeriodSubtotalMinor})
	}
	prices, err := a.pricing.EvaluatePrices(ctx, request)
	if err != nil {
		return nil, err
	}
	result := &QuoteAdjustmentResult{Adjustments: []QuoteAdjustment{}}
	matched := make(map[string]bool, len(prices))
	for _, price := range prices {
		matched[price.ReferenceNo] = true
		original, target := price.OriginalAmountMinor, price.TargetAmountMinor
		result.Adjustments = append(result.Adjustments, QuoteAdjustment{ReferenceNo: price.ReferenceNo, TypeCode: QuoteAdjustmentAgentPrice, SourceReference: price.PriceNo + ":" + strconv.FormatInt(price.Version, 10), SourceVersionDigest: price.ContentDigest, AmountMinor: 0, OriginalPeriodAmountMinor: &original, TargetPeriodAmountMinor: &target})
	}
	marketingInput := input
	marketingInput.Lines = make([]QuoteAdjustmentLineInput, 0, len(input.Lines))
	for _, line := range input.Lines {
		if !matched[line.ReferenceNo] {
			marketingInput.Lines = append(marketingInput.Lines, line)
		}
	}
	if len(marketingInput.Lines) == 0 {
		return result, nil
	}
	benefits, err := a.marketing.EvaluateAndReserveQuote(ctx, marketingInput)
	if err != nil {
		return nil, err
	}
	if benefits == nil {
		return nil, gerror.New("marketing quote returned no result")
	}
	result.Adjustments = append(result.Adjustments, benefits.Adjustments...)
	result.ExpiresAt = benefits.ExpiresAt
	return result, nil
}
func (a *agentQuoteAdjustmentEvaluator) ConsumeQuote(ctx context.Context, customer, quote string) error {
	if owner, ok := a.marketing.(QuoteReservationFinalizer); ok {
		return owner.ConsumeQuote(ctx, customer, quote)
	}
	return nil
}
func (a *agentQuoteAdjustmentEvaluator) ReverseConsumedQuote(ctx context.Context, customer, quote string) error {
	if owner, ok := a.marketing.(QuoteReservationReverser); ok {
		return owner.ReverseConsumedQuote(ctx, customer, quote)
	}
	return nil
}

// agentPriceBasis excludes explicit setup fees while keeping one-time product
// charges eligible. Existing accounting classifies one-time charges as setup;
// this pricing-only projection must not confuse them with installation fees.
func agentPriceBasis(line catalog.PurchaseQuoteLine) (int64, error) {
	pricingLine := line
	if pricingLine.PeriodUnitCode == string(catalog.PeriodUnitOnce) {
		pricingLine.PeriodUnitCode = string(catalog.PeriodUnitMonth)
	}
	components, err := quoteTaxExclusiveComponents(pricingLine)
	if err != nil {
		return 0, err
	}
	var amount int64
	for _, component := range components {
		if !component.setup {
			amount, err = safeQuoteAmountAdd(amount, component.amount)
			if err != nil {
				return 0, err
			}
		}
	}
	return amount, nil
}
