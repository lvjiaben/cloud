package public

import (
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func humanChallengeProjection(item *notification.PresalesHumanChallenge) *v1.HumanChallengePresentation {
	if item == nil {
		return nil
	}
	return &v1.HumanChallengePresentation{ChallengeNo: item.ChallengeNo, ProviderCode: item.ProviderCode,
		ProviderAction: item.ProviderAction, Kind: item.Kind, ImageDataURL: item.ImageDataURL,
		AccessiblePrompt: item.AccessiblePrompt, SiteKey: item.SiteKey, ExpiresAt: item.ExpiresAt.UTC().UnixMilli()}
}

func verificationProjection(item *notification.PresalesVerification) *v1.VerificationCodeProjection {
	if item == nil {
		return nil
	}
	return &v1.VerificationCodeProjection{VerificationNo: item.VerificationNo, MaskedDestination: item.MaskedDestination,
		DeliveryState: item.DeliveryState, ExpiresAt: item.ExpiresAt.UTC().UnixMilli(), RetryAfterAt: item.RetryAfterAt.UTC().UnixMilli()}
}

func humanProofProjection(item *notification.PresalesHumanProof) *v1.HumanChallengeProof {
	if item == nil {
		return nil
	}
	return &v1.HumanChallengeProof{ProofToken: item.ProofToken, ExpiresAt: item.ExpiresAt.UTC().UnixMilli()}
}

func accessProjection(item *notification.PresalesAccess) *v1.PresalesAccessProjection {
	if item == nil {
		return nil
	}
	return &v1.PresalesAccessProjection{TicketNo: item.TicketNo, AccessToken: item.AccessToken,
		ExpiresAt: item.ExpiresAt.UTC().UnixMilli(), LinkDeliveryState: item.LinkDeliveryState}
}

func notificationUnavailable() error { return bizerr.NewCode(notification.CodeUnavailable) }
