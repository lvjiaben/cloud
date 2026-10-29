// This file adapts customer authentication to bounded public plugin services.
package public

import (
	"context"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// RecoveryHumanCompletionCreate preserves host identity ownership and does not return contact projections.
func (c *ControllerV1) RecoveryHumanCompletionCreate(ctx context.Context, req *v1.RecoveryHumanCompletionCreateReq) (*v1.RecoveryHumanCompletionCreateRes, error) {
	input := notification.RecoveryCompletionInput{RecoveryInput: notification.RecoveryInput{Kind: customerlogincap.Kind(req.Kind), Account: req.Account, BrowserNonce: req.BrowserNonce, Locale: req.Locale}, ChallengeNo: req.ChallengeNo, Answer: req.Answer, ProviderToken: req.ProviderToken}
	req.Answer, req.ProviderToken = "", ""
	item, err := c.notifications.CompleteRecoveryHumanChallenge(ctx, input)
	if err != nil {
		return nil, err
	}
	return (*v1.RecoveryHumanCompletionCreateRes)(humanProofProjection(item)), nil
}
