package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingValueGrantAdjustmentReview(ctx context.Context, req *v1.MarketingValueGrantAdjustmentReviewReq) (res *v1.MarketingValueGrantAdjustmentReviewRes, err error) {
	result, err := c.marketingActions.ReviewValueGrantAdjustment(ctx, marketing.ValueGrantFinancialReviewInput{
		AdjustmentNo: req.AdjustmentNo, Decision: req.Decision, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingValueGrantAdjustmentReviewRes{
		Mutation: toAPIManagedValueGrantFinancialMutation(result),
	}, nil
}
