// This file retries only the session cleanup authorized by a completed reset.
package public

import (
	"context"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// RecoverySessionCleanup never changes passwords or exposes the temporary token.
func (c *ControllerV1) RecoverySessionCleanup(ctx context.Context, req *v1.RecoverySessionCleanupReq) (*v1.RecoverySessionCleanupRes, error) {
	input := notification.RecoveryCleanupInput{RecoveryInput: notification.RecoveryInput{Kind: customerlogincap.Kind(req.Kind), Account: req.Account, BrowserNonce: req.BrowserNonce, Locale: req.Locale}, VerificationNo: req.VerificationNo, CleanupToken: req.CleanupToken}
	req.CleanupToken = ""
	pending, err := c.notifications.RetryRecoveryCleanup(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.RecoverySessionCleanupRes{Pending: pending}, nil
}
