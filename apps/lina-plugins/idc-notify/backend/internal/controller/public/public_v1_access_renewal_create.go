package public

import (
	"context"

	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) AccessRenewalCreate(ctx context.Context, req *v1.AccessRenewalCreateReq) (res *v1.AccessRenewalCreateRes, err error) {
	browser, destination, code := []byte(req.BrowserNonce), []byte(req.Destination), []byte(req.Code)
	req.BrowserNonce, req.Destination, req.Code = "", "", ""
	item, err := c.notifications.RenewAnonymousPresalesAccess(ctx, notification.PresalesRenewalInput{
		BrowserNonce: browser, Destination: destination, DestinationKind: req.DestinationKind, Locale: req.Locale,
		OperationNo: req.OperationNo, VerificationNo: req.VerificationNo, Code: code, TicketNo: req.TicketNo,
	})
	if err != nil {
		return nil, err
	}
	return (*v1.AccessRenewalCreateRes)(accessProjection(item)), nil
}
