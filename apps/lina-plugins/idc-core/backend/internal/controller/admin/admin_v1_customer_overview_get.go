package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerOverviewGet(ctx context.Context, req *v1.CustomerOverviewGetReq) (res *v1.CustomerOverviewGetRes, err error) {
	overview, err := c.customerManagement.GetOverview(ctx, req.CustomerNo)
	if err != nil {
		return nil, err
	}
	return toAPIOverview(overview), nil
}
