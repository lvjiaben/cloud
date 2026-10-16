// This file returns exact customer-isolated call history with safe millisecond timestamps.
package admin

import (
	"context"
	"lina-plugin-idc-channel/backend/api/admin/v1"
	platformv1 "lina-plugin-idc-channel/backend/api/platform/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

// CustomerAPICalls pages all retained calls, excluding bodies and credentials.
func (c *ControllerV1) CustomerAPICalls(ctx context.Context, req *v1.CustomerAPICallsReq) (*v1.CustomerAPICallsRes, error) {
	page, err := c.service.StaffCalls(ctx, req.CustomerNo, channel.PageQuery{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	result := &v1.CustomerAPICallsRes{CallsRes: platformv1.CallsRes{List: []platformv1.Call{}, Total: page.Total}}
	for _, row := range page.List {
		result.List = append(result.List, platformv1.Call{ID: row.ID, Method: row.Method, Operation: row.Path, Succeeded: row.Succeeded, DurationMillis: row.DurationMillis, CreatedAt: row.CreatedAt.UnixMilli()})
	}
	return result, nil
}
