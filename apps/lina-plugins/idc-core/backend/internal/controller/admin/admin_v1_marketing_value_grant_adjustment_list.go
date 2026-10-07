package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingValueGrantAdjustmentList(
	ctx context.Context,
	req *v1.MarketingValueGrantAdjustmentListReq,
) (*v1.MarketingValueGrantAdjustmentListRes, error) {
	page, err := c.marketingActions.ListValueGrantAdjustments(ctx, marketing.ValueGrantFinancialAdjustmentSearchInput{
		Page:   req.Page,
		Cursor: req.Cursor, Limit: req.Limit, Status: req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingValueGrantFinancialAdjustment, 0, len(page.Adjustments))
	for index := range page.Adjustments {
		item := page.Adjustments[index]
		items = append(items, &v1.MarketingValueGrantFinancialAdjustment{
			MarketingValueGrantFinancialMutation: *toAPIManagedValueGrantFinancialMutation(
				&item.ManagedValueGrantFinancialMutation,
			),
			RequestedBy: item.RequestedBy, CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return &v1.MarketingValueGrantAdjustmentListRes{
		Total:       page.Total,
		Adjustments: items, NextCursor: page.NextCursor,
	}, nil
}
