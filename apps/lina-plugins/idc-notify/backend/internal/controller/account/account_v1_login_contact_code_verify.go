// This file adapts one session-bound login-contact code verification step.
package account

import (
	"context"
	"lina-plugin-idc-notify/backend/api/account/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// LoginContactCodeVerify verifies a code without changing login credentials.
func (c *ControllerV1) LoginContactCodeVerify(ctx context.Context, req *v1.LoginContactCodeVerifyReq) (*v1.LoginContactCodeVerifyRes, error) {
	err := c.notifications.VerifyLoginContactCode(ctx, notification.LoginContactVerifyInput{LoginContactInput: notification.LoginContactInput{Kind: notification.ContactKind(req.Kind), OldVerification: req.OldVerification, Destination: req.Destination, Locale: req.Locale}, VerificationNo: req.VerificationNo, Code: req.Code})
	if err != nil {
		return nil, err
	}
	return &v1.LoginContactCodeVerifyRes{}, nil
}
