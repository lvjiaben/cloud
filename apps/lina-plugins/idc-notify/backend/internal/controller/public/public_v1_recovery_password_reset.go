// This file adapts customer authentication to bounded public plugin services.
package public

import (
	"context"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// RecoveryPasswordReset preserves host identity ownership and does not return contact projections.
func (c *ControllerV1) RecoveryPasswordReset(ctx context.Context, req *v1.RecoveryPasswordResetReq) (*v1.RecoveryPasswordResetRes, error) {
	input := notification.RecoveryResetInput{RecoveryInput: notification.RecoveryInput{Kind: customerlogincap.Kind(req.Kind), Account: req.Account, BrowserNonce: req.BrowserNonce, Locale: req.Locale}, VerificationNo: req.VerificationNo, Code: req.Code, Password: req.Password}
	req.Code, req.Password = "", ""
	item, err := c.notifications.ResetCustomerPassword(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.RecoveryPasswordResetRes{Reset: item.Reset, CleanupPending: item.CleanupPending, CleanupToken: item.CleanupToken}, nil
}
