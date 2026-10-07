package public

import (
	"context"

	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) HumanChallengeCreate(ctx context.Context, req *v1.HumanChallengeCreateReq) (res *v1.HumanChallengeCreateRes, err error) {
	browser, destination := []byte(req.BrowserNonce), []byte(req.Destination)
	req.BrowserNonce, req.Destination = "", ""
	item, err := c.notifications.CreatePresalesHumanChallenge(ctx, notification.PresalesHumanChallengeInput{
		PresalesBindingInput: notification.PresalesBindingInput{BrowserNonce: browser, Destination: destination,
			DestinationKind: req.DestinationKind, Locale: req.Locale, Operation: req.Operation, TicketNo: req.TicketNo},
		PresentationKind: req.PresentationKind,
	})
	if err != nil {
		return nil, err
	}
	return (*v1.HumanChallengeCreateRes)(humanChallengeProjection(item)), nil
}
