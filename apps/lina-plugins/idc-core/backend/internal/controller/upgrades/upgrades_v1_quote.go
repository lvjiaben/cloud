// This file delegates the authenticated quote action without accepting caller-supplied money.
package upgrades

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/upgrades/v1"
)

// Quote reuses the shared runtime ownership and revision checks.
func (c *ControllerV1) Quote(ctx context.Context, req *v1.QuoteReq) (*v1.QuoteRes, error) {
	r, err := c.service.Quote(ctx, req.EntitlementNo, req.QuoteNo, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &v1.QuoteRes{Upgrade: project(r)}, nil
}
