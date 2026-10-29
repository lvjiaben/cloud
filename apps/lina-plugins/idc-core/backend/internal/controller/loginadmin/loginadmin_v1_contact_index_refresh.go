// This file refreshes one permission-scoped customer login index page.
package loginadmin

import (
	"context"
	"lina-plugin-idc-core/backend/api/loginadmin/v1"
)

func (c *ControllerV1) ContactIndexRefresh(ctx context.Context, req *v1.ContactIndexRefreshReq) (*v1.ContactIndexRefreshRes, error) {
	result, err := c.directory.Sync(ctx, req.PageNum, req.RepairCustomerAccess)
	if err != nil {
		return nil, err
	}
	return &v1.ContactIndexRefreshRes{Refreshed: result.Refreshed, Total: result.Total, RepairCandidateUserIDs: result.RepairCandidateUserIDs, AccessRepaired: result.AccessRepaired}, nil
}
