package admin

import (
	"context"
	"strconv"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingValueGrantReverseRequest(ctx context.Context, req *v1.MarketingValueGrantReverseRequestReq) (res *v1.MarketingValueGrantReverseRequestRes, err error) {
	amountMinor, err := strconv.ParseInt(req.AmountMinor, 10, 64)
	if err != nil {
		amountMinor = 0
	}
	result, err := c.marketingActions.RequestValueGrantReversal(ctx, marketing.ValueGrantFinancialRequestInput{
		RequestNo: req.RequestNo, GrantNo: req.GrantNo,
		AmountMinor: amountMinor, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingValueGrantReverseRequestRes{
		Mutation: toAPIManagedValueGrantFinancialMutation(result),
	}, nil
}
