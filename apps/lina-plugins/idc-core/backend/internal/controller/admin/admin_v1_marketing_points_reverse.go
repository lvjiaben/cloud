package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingPointsReverse(ctx context.Context, req *v1.MarketingPointsReverseReq) (*v1.MarketingPointsReverseRes, error) {
	result, err := c.marketingActions.ReversePointsAward(ctx, marketing.PointsReverseManagementInput{
		RequestNo: req.RequestNo, CustomerNo: req.CustomerNo,
		AwardSourceType: req.AwardSourceType, AwardSourceReference: req.AwardSourceReference,
		Points: req.Points, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingPointsReverseRes{Mutation: toAPIManagedPointsMutation(result)}, nil
}
