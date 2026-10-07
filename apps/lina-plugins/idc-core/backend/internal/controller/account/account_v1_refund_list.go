// This file exposes the current-customer bounded refund directory.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) RefundList(
	ctx context.Context,
	req *v1.RefundListReq,
) (*v1.RefundListRes, error) {
	page, err := c.refunds.ListOnlineRefunds(ctx, billing.RefundDirectoryFilter{
		Status: req.StatusCode, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	result := &v1.RefundListRes{
		Refunds: make([]*v1.Refund, 0, len(page.Refunds)), NextCursor: page.NextCursor,
	}
	for _, refund := range page.Refunds {
		result.Refunds = append(result.Refunds, toAPIRefund(refund))
	}
	return result, nil
}
