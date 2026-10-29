// This file adapts customer authentication to bounded public plugin services.
package public

import (
	"context"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// RecoveryHumanChallengeCreate preserves host identity ownership and does not return contact projections.
func (c *ControllerV1) RecoveryHumanChallengeCreate(ctx context.Context, req *v1.RecoveryHumanChallengeCreateReq) (*v1.RecoveryHumanChallengeCreateRes, error) {
	item, err := c.notifications.CreateRecoveryHumanChallenge(ctx, notification.RecoveryChallengeInput{RecoveryInput: notification.RecoveryInput{Kind: customerlogincap.Kind(req.Kind), Account: req.Account, BrowserNonce: req.BrowserNonce, Locale: req.Locale}, PresentationKind: req.PresentationKind})
	if err != nil {
		return nil, err
	}
	return (*v1.RecoveryHumanChallengeCreateRes)(humanChallengeProjection(item)), nil
}
