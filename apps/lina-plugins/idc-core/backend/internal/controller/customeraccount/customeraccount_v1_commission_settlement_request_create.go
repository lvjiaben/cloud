package customeraccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

func (c *ControllerV1) CommissionSettlementRequestCreate(ctx context.Context, req *v1.CommissionSettlementRequestCreateReq) (res *v1.CommissionSettlementRequestCreateRes, err error) {
	profile, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	result, err := c.salesChannel.RequestCustomerCommissionSettlement(ctx, profile.Subject.CustomerNo, saleschannel.CustomerCommissionSettlementRequest{CurrencyCode: req.CurrencyCode, AmountMinor: req.AmountMinor, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.CommissionSettlementRequestCreateRes{Item: &v1.CustomerCommissionSettlement{SettlementNo: result.Item.SettlementNo, CurrencyCode: result.Item.CurrencyCode, AmountMinor: result.Item.AmountMinor, StatusCode: result.Item.StatusCode, SubmittedAt: result.Item.SubmittedAt, ReviewedAt: result.Item.ReviewedAt, PaidAt: result.Item.PaidAt, FailureReasonCode: result.Item.FailureReasonCode}, Reused: result.Reused}, nil
}
