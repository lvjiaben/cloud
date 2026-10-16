// 本文件将后台更新客户授权状态，沿用原服务的状态校验。

package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

// SetChannelStatus 更新客户授权状态，沿用原服务的状态校验。
func (c *ControllerV1) SetChannelStatus(ctx context.Context, req *v1.SetChannelStatusReq) (*v1.SetChannelStatusRes, error) {
	return c.delegate.SetChannelStatus(ctx, req)
}
