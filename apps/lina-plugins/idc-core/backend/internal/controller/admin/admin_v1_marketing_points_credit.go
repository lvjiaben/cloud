package admin

import (
	"context"
	"time"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingPointsCredit(ctx context.Context, req *v1.MarketingPointsCreditReq) (*v1.MarketingPointsCreditRes, error) {
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		value := time.UnixMilli(*req.ExpiresAt).UTC()
		expiresAt = &value
	}
	result, err := c.marketingActions.CreditPoints(ctx, marketing.PointsCreditInput{
		RequestNo: req.RequestNo, CustomerNo: req.CustomerNo, Points: req.Points,
		ExpiresAt: expiresAt, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingPointsCreditRes{Mutation: toAPIManagedPointsMutation(result)}, nil
}
