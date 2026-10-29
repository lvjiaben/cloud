// This file adapts authenticated login-contact code delivery to the public API.
package account

import (
	"context"
	"lina-plugin-idc-notify/backend/api/account/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// LoginContactCodeCreate sends one old or new contact verification message.
func (c *ControllerV1) LoginContactCodeCreate(ctx context.Context, req *v1.LoginContactCodeCreateReq) (*v1.LoginContactCodeCreateRes, error) {
	result, err := c.notifications.SendLoginContactCode(ctx, notification.LoginContactInput{Kind: notification.ContactKind(req.Kind), OldVerification: req.OldVerification, Destination: req.Destination, Locale: req.Locale, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.LoginContactCodeCreateRes{VerificationNo: result.VerificationNo, MaskedDestination: result.MaskedDestination, DeliveryState: result.DeliveryState, ExpiresAt: result.ExpiresAt.UnixMilli(), RetryAfterAt: result.RetryAfterAt.UnixMilli()}, nil
}
