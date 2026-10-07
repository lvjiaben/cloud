package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
)

// ProviderTaskDetail returns one bounded local reconciliation projection.
func (c *ControllerV1) ProviderTaskDetail(ctx context.Context, req *v1.ProviderTaskDetailReq) (res *v1.ProviderTaskDetailRes, err error) {
	item, err := c.tasks.Detail(ctx, req.TaskNo)
	if err != nil {
		return nil, err
	}
	return &v1.ProviderTaskDetailRes{Item: providerTaskDetail(item)}, nil
}
