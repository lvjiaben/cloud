package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
)

func (c *ControllerV1) CustomerActivityList(ctx context.Context, req *v1.CustomerActivityListReq) (*v1.CustomerActivityListRes, error) {
	page, err := c.customerManagement.SearchActivities(ctx, customermanagement.ActivitySearchInput{
		CustomerNo: req.CustomerNo, Kind: customermanagement.ActivityKind(req.Kind), Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CustomerActivityItem, 0, len(page.Items))
	for _, item := range page.Items {
		plannedAt := int64(0)
		if item.PlannedAt != nil {
			plannedAt = item.PlannedAt.UnixMilli()
		}
		items = append(items, &v1.CustomerActivityItem{Number: item.Number, Kind: string(item.Kind),
			Category: item.Category, Status: item.Status, Title: item.Title, Detail: item.Detail,
			RiskLevel: item.RiskLevel, PlannedAt: plannedAt,
			CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli()})
	}
	return &v1.CustomerActivityListRes{Items: items, NextCursor: page.NextCursor}, nil
}
