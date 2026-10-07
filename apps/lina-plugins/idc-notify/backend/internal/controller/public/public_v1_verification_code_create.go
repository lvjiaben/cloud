package public

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) VerificationCodeCreate(ctx context.Context, req *v1.VerificationCodeCreateReq) (res *v1.VerificationCodeCreateRes, err error) {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, notificationUnavailable()
	}
	browser, destination, clientIP := []byte(req.BrowserNonce), []byte(req.Destination), []byte(request.GetClientIp())
	req.BrowserNonce, req.Destination = "", ""
	item, err := c.notifications.SendPresalesVerificationCode(ctx, notification.PresalesVerificationInput{
		PresalesBindingInput: notification.PresalesBindingInput{BrowserNonce: browser, Destination: destination,
			DestinationKind: req.DestinationKind, Locale: req.Locale, Operation: req.Operation, TicketNo: req.TicketNo},
		ProofToken: req.ProofToken, IdempotencyKey: req.IdempotencyKey, ClientIP: clientIP,
	})
	if err != nil {
		return nil, err
	}
	return (*v1.VerificationCodeCreateRes)(verificationProjection(item)), nil
}
