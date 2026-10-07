package customeraccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
)

func (c *ControllerV1) CommissionSettlementsList(ctx context.Context, req *v1.CommissionSettlementsListReq) (res *v1.CommissionSettlementsListRes, err error) {
	profile, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	page, err := c.salesChannel.ListCustomerCommissionSettlements(ctx, profile.Subject.CustomerNo, req.CurrencyCode, req.Limit)
	if err != nil {
		return nil, err
	}
	res = &v1.CommissionSettlementsListRes{Items: make([]*v1.CustomerCommissionSettlement, 0, len(page.Items))}
	for _, item := range page.Items {
		res.Items = append(res.Items, &v1.CustomerCommissionSettlement{SettlementNo: item.SettlementNo, CurrencyCode: item.CurrencyCode, AmountMinor: item.AmountMinor, StatusCode: item.StatusCode, SubmittedAt: item.SubmittedAt, ReviewedAt: item.ReviewedAt, PaidAt: item.PaidAt, FailureReasonCode: item.FailureReasonCode})
	}
	return res, nil
}
