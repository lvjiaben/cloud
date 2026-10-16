// This file delegates the authenticated pay action without accepting caller-supplied money.
package upgrades

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/upgrades/v1"
)

// Pay reuses the shared runtime ownership and revision checks.
func (c *ControllerV1) Pay(ctx context.Context, req *v1.PayReq) (*v1.PayRes, error) {
	r, err := c.service.Pay(ctx, req.UpgradeNo)
	if err != nil {
		return nil, err
	}
	return &v1.PayRes{Upgrade: project(r)}, nil
}
