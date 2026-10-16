// This file projects customer-isolated call metadata with Unix millisecond timestamps.
package platform

import (
	"context"
	"lina-plugin-idc-channel/backend/api/platform/v1"
	channelservice "lina-plugin-idc-channel/backend/internal/service/channel"
)

// Calls returns one bounded page and its exact customer-scoped total.
func (c *ControllerV1) Calls(ctx context.Context, req *v1.CallsReq) (*v1.CallsRes, error) {
	result, err := c.service.OwnCalls(ctx, channelservice.PageQuery{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	rows := make([]v1.Call, 0, len(result.List))
	for _, row := range result.List {
		rows = append(rows, v1.Call{ID: row.ID, Method: row.Method, Operation: row.Path, Succeeded: row.Succeeded, DurationMillis: row.DurationMillis, CreatedAt: row.CreatedAt.UnixMilli()})
	}
	return &v1.CallsRes{List: rows, Total: result.Total}, nil
}
