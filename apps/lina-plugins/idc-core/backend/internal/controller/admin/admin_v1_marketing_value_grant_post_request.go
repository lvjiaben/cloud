package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingValueGrantPostRequest(ctx context.Context, req *v1.MarketingValueGrantPostRequestReq) (res *v1.MarketingValueGrantPostRequestRes, err error) {
	result, err := c.marketingActions.RequestValueGrantPosting(ctx, marketing.ValueGrantFinancialRequestInput{
		RequestNo: req.RequestNo, GrantNo: req.GrantNo, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingValueGrantPostRequestRes{
		Mutation: toAPIManagedValueGrantFinancialMutation(result),
	}, nil
}
