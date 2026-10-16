// 本文件将后台分页读取客户授权，沿用原服务权限和范围。

package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

// ListChannels 分页读取客户授权，沿用原服务权限和范围。
func (c *ControllerV1) ListChannels(ctx context.Context, req *v1.ListChannelsReq) (*v1.ListChannelsRes, error) {
	return c.delegate.ListChannels(ctx, req)
}
