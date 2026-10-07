package commerce

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/marketing"
)

type marketingQuoteAdjustmentEvaluator struct {
	service marketing.Service
}

var _ QuoteAdjustmentEvaluator = (*marketingQuoteAdjustmentEvaluator)(nil)
var _ QuoteReservationFinalizer = (*marketingQuoteAdjustmentEvaluator)(nil)
var _ QuoteReservationReverser = (*marketingQuoteAdjustmentEvaluator)(nil)

// NewMarketingQuoteAdjustmentEvaluator adapts the marketing owner's quote-wide
// reservation contract without exposing marketing persistence to commerce.
func NewMarketingQuoteAdjustmentEvaluator(service marketing.Service) (QuoteAdjustmentEvaluator, error) {
	if service == nil {
		return nil, gerror.New("marketing quote adjustment evaluator requires a service")
	}
	return &marketingQuoteAdjustmentEvaluator{service: service}, nil
}

func (a *marketingQuoteAdjustmentEvaluator) EvaluateAndReserveQuote(
	ctx context.Context,
	input QuoteAdjustmentInput,
) (*QuoteAdjustmentResult, error) {
	if (input.PurchasePhase != marketing.PurchasePhaseNewPurchase &&
		input.PurchasePhase != marketing.PurchasePhaseRenewal) || input.CycleNumber < 1 {
		return nil, gerror.New("marketing quote adjustment phase is invalid")
	}
	request := marketing.QuoteReserveInput{
		CustomerNo: input.CustomerNo, QuoteReference: input.QuoteNo,
		CurrencyCode: input.CurrencyCode, PurchasePhase: input.PurchasePhase,
		CycleNumber: input.CycleNumber, AssetNumbers: append([]string(nil), input.AssetNumbers...),
		Lines: make([]marketing.QuoteLineInput, 0, len(input.Lines)),
	}
	for _, line := range input.Lines {
		request.Lines = append(request.Lines, marketing.QuoteLineInput{
			ReferenceNo: line.ReferenceNo, ProductNo: line.ProductNo,
			CategoryNo:    line.CategoryNo,
			SubtotalMinor: line.SubtotalMinor,
		})
	}
	reserved, err := a.service.ReserveQuote(ctx, request)
	if err != nil {
		return nil, err
	}
	result := &QuoteAdjustmentResult{Adjustments: []QuoteAdjustment{}}
	if reserved.ExpiresAt != nil {
		expiresAt := *reserved.ExpiresAt
		result.ExpiresAt = &expiresAt
	}
	for _, benefit := range reserved.Applied {
		allocatedTotal := int64(0)
		for _, allocation := range benefit.Allocations {
			allocatedTotal, err = safeQuoteAmountAdd(allocatedTotal, allocation.AmountMinor)
			if err != nil {
				return nil, err
			}
			result.Adjustments = append(result.Adjustments, QuoteAdjustment{
				ReferenceNo:          allocation.LineReference,
				TypeCode:             marketingQuoteAdjustmentType(benefit),
				SourceReference:      benefit.CampaignNo + ":" + benefit.BenefitNo,
				SourceVersionDigest:  benefit.BenefitSnapshotDigest,
				ReservationReference: benefit.ReservationReference,
				AmountMinor:          allocation.AmountMinor,
			})
		}
		if allocatedTotal != benefit.DiscountAmountMinor {
			return nil, gerror.New("marketing quote allocation total is inconsistent")
		}
	}
	return result, nil
}

// ConsumeQuote converts quote-time campaign and asset holds into consumed quota.
// The marketing service joins the caller transaction, so a later checkout error
// rolls this transition back together with the order and invoice.
func (a *marketingQuoteAdjustmentEvaluator) ConsumeQuote(
	ctx context.Context,
	customerNo string,
	quoteNo string,
) error {
	_, err := a.service.Consume(ctx, customerNo, quoteNo)
	return err
}

// ReverseConsumedQuote returns unpaid-order campaign and asset quota inside
// the caller's cancellation transaction.
func (a *marketingQuoteAdjustmentEvaluator) ReverseConsumedQuote(
	ctx context.Context,
	customerNo string,
	quoteNo string,
) error {
	_, err := a.service.ReverseConsumed(ctx, customerNo, quoteNo)
	return err
}

func marketingQuoteAdjustmentType(benefit marketing.QuoteAppliedBenefit) string {
	if benefit.BenefitType == "customer_price" {
		return QuoteAdjustmentCustomerPrice
	}
	if benefit.BenefitType == "voucher" {
		return QuoteAdjustmentVoucher
	}
	if benefit.AssetNo != "" {
		return QuoteAdjustmentCoupon
	}
	return QuoteAdjustmentCampaignDiscount
}
