// This file adapts customer authentication to bounded public plugin services.
package public

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// RecoveryCodeCreate preserves host identity ownership and does not return contact projections.
func (c *ControllerV1) RecoveryCodeCreate(ctx context.Context, req *v1.RecoveryCodeCreateReq) (*v1.RecoveryCodeCreateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, notificationUnavailable()
	}
	item, err := c.notifications.SendRecoveryCode(ctx, notification.RecoveryCodeInput{RecoveryInput: notification.RecoveryInput{Kind: customerlogincap.Kind(req.Kind), Account: req.Account, BrowserNonce: req.BrowserNonce, Locale: req.Locale}, ProofToken: req.ProofToken, IdempotencyKey: req.IdempotencyKey, ClientIP: request.GetClientIp()})
	if err != nil {
		return nil, err
	}
	return (*v1.RecoveryCodeCreateRes)(verificationProjection(item)), nil
}
