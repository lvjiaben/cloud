package public

import (
	"context"

	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) HumanChallengeCompletionCreate(ctx context.Context, req *v1.HumanChallengeCompletionCreateReq) (res *v1.HumanChallengeCompletionCreateRes, err error) {
	browser, destination := []byte(req.BrowserNonce), []byte(req.Destination)
	answer, providerToken := []byte(req.Answer), []byte(req.ProviderToken)
	req.BrowserNonce, req.Destination, req.Answer, req.ProviderToken = "", "", "", ""
	item, err := c.notifications.CompletePresalesHumanChallenge(ctx, notification.PresalesHumanCompletionInput{
		PresalesBindingInput: notification.PresalesBindingInput{BrowserNonce: browser, Destination: destination,
			DestinationKind: req.DestinationKind, Locale: req.Locale, Operation: req.Operation, TicketNo: req.TicketNo},
		ChallengeNo: req.ChallengeNo, Answer: answer, ProviderToken: providerToken,
	})
	if err != nil {
		return nil, err
	}
	return &v1.HumanChallengeCompletionCreateRes{ProofToken: item.ProofToken, ExpiresAt: item.ExpiresAt.UTC().UnixMilli()}, nil
}
