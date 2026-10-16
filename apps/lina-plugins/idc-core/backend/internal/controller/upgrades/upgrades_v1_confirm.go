// This file delegates the authenticated confirm action without accepting caller-supplied money.
package upgrades

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/upgrades/v1"
)

// Confirm reuses the shared runtime ownership and revision checks.
func (c *ControllerV1) Confirm(ctx context.Context, req *v1.ConfirmReq) (*v1.ConfirmRes, error) {
	r, err := c.service.Confirm(ctx, req.UpgradeNo)
	if err != nil {
		return nil, err
	}
	return &v1.ConfirmRes{Upgrade: project(r)}, nil
}
