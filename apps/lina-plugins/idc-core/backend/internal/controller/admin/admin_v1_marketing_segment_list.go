// This file handles the bounded marketing segment queue endpoint.

package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingSegmentList(ctx context.Context, req *v1.MarketingSegmentListReq) (*v1.MarketingSegmentListRes, error) {
	page, err := c.marketingDirectory.SearchSegments(ctx, marketing.SegmentManagementSearchInput{
		Page:       req.Page,
		NamePrefix: req.NamePrefix, Mode: req.Mode, Status: req.Status, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingSegmentSummaryItem, 0, len(page.Segments))
	for _, item := range page.Segments {
		items = append(items, toAPIMarketingSegmentSummary(item))
	}
	return &v1.MarketingSegmentListRes{Segments: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
