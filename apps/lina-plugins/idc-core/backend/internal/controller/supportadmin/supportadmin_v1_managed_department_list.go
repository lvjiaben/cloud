// This file exposes only the current supervisor managed queues.

package supportadmin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SupportManagedDepartmentList(ctx context.Context, req *v1.SupportManagedDepartmentListReq) (res *v1.SupportManagedDepartmentListRes, err error) {
	items, err := c.service.ManagedDepartments(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SupportManagedDepartmentListRes{Items: toDepartmentSettings(items)}, nil
}
