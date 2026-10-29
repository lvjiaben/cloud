// This file retries cleanup only for the actual authenticated customer.
package account

import (
	"context"
	"lina-plugin-idc-notify/backend/api/account/v1"
)

// RecoverySessionCleanup keeps cleanup failures separate from password success.
func (c *ControllerV1) RecoverySessionCleanup(ctx context.Context, req *v1.RecoverySessionCleanupReq) (*v1.RecoverySessionCleanupRes, error) {
	pending, err := c.notifications.CleanupRecoverySessions(ctx, req.RecoveryNo)
	if err != nil {
		return nil, err
	}
	return &v1.RecoverySessionCleanupRes{Pending: pending}, nil
}
