// This file handles the bounded marketing segment detail endpoint.

package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) MarketingSegmentDetail(ctx context.Context, req *v1.MarketingSegmentDetailReq) (*v1.MarketingSegmentDetailRes, error) {
	detail, err := c.marketingDirectory.GetSegment(ctx, req.SegmentNo)
	if err != nil {
		return nil, err
	}
	return toAPIMarketingSegmentDetail(detail), nil
}
