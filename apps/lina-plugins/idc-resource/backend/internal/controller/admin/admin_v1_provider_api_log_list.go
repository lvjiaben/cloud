// This file maps authorized local invocation history to secret-free API rows.
package admin

import (
	"context"
	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// ProviderAPILogList returns one local page, with no Provider calls.
func (c *ControllerV1) ProviderAPILogList(ctx context.Context, req *v1.ProviderAPILogListReq) (*v1.ProviderAPILogListRes, error) {
	page, err := c.tasks.ListAPILogs(ctx, resourcetask.APILogListInput{Page: req.Page, Limit: req.Limit, ChannelNo: req.ChannelNo, ChannelName: req.ChannelName, MethodCode: req.MethodCode, Result: req.Result})
	if err != nil {
		return nil, err
	}
	result := &v1.ProviderAPILogListRes{Total: page.Total, Items: make([]*v1.ProviderAPILogItem, len(page.Items))}
	for i, row := range page.Items {
		started := int64(0)
		if row.StartedAt != nil {
			started = row.StartedAt.UnixMilli()
		}
		result.Items[i] = &v1.ProviderAPILogItem{ID: row.Id, ChannelNo: row.ChannelNo, ChannelName: row.ChannelName, ProviderPluginID: row.ProviderPluginId, MethodCode: row.MethodCode, Succeeded: row.Succeeded, ErrorCode: row.ErrorCode, DurationMS: row.DurationMs, StartedAt: started}
	}
	return result, nil
}
