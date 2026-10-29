package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// ProviderTaskDetail returns one bounded local reconciliation projection.
func (c *ControllerV1) ProviderTaskDetail(ctx context.Context, req *v1.ProviderTaskDetailReq) (res *v1.ProviderTaskDetailRes, err error) {
	item, err := c.tasks.Detail(ctx, req.TaskNo, resourcetask.TaskScope{ServiceNo: req.ServiceNo, ChannelNo: req.ChannelNo})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderTaskDetailRes{Item: providerTaskDetail(item)}, nil
}
