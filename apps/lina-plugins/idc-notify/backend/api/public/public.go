// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package public

import (
	"context"

	"lina-plugin-idc-notify/backend/api/public/v1"
)

type IPublicV1 interface {
	AccessRenewalCreate(ctx context.Context, req *v1.AccessRenewalCreateReq) (res *v1.AccessRenewalCreateRes, err error)
	CustomerPasswordLogin(ctx context.Context, req *v1.CustomerPasswordLoginReq) (res *v1.CustomerPasswordLoginRes, err error)
	RecoveryHumanChallengeCreate(ctx context.Context, req *v1.RecoveryHumanChallengeCreateReq) (res *v1.RecoveryHumanChallengeCreateRes, err error)
	RecoveryHumanCompletionCreate(ctx context.Context, req *v1.RecoveryHumanCompletionCreateReq) (res *v1.RecoveryHumanCompletionCreateRes, err error)
	RecoveryCodeCreate(ctx context.Context, req *v1.RecoveryCodeCreateReq) (res *v1.RecoveryCodeCreateRes, err error)
	RecoveryPasswordReset(ctx context.Context, req *v1.RecoveryPasswordResetReq) (res *v1.RecoveryPasswordResetRes, err error)
	RecoverySessionCleanup(ctx context.Context, req *v1.RecoverySessionCleanupReq) (res *v1.RecoverySessionCleanupRes, err error)
	HumanChallengeCompletionCreate(ctx context.Context, req *v1.HumanChallengeCompletionCreateReq) (res *v1.HumanChallengeCompletionCreateRes, err error)
	HumanChallengeCreate(ctx context.Context, req *v1.HumanChallengeCreateReq) (res *v1.HumanChallengeCreateRes, err error)
	OnboardingHumanChallengeCreate(ctx context.Context, req *v1.OnboardingHumanChallengeCreateReq) (res *v1.OnboardingHumanChallengeCreateRes, err error)
	OnboardingHumanChallengeCompletionCreate(ctx context.Context, req *v1.OnboardingHumanChallengeCompletionCreateReq) (res *v1.OnboardingHumanChallengeCompletionCreateRes, err error)
	OnboardingVerificationCodeCreate(ctx context.Context, req *v1.OnboardingVerificationCodeCreateReq) (res *v1.OnboardingVerificationCodeCreateRes, err error)
	OnboardingIntentCreate(ctx context.Context, req *v1.OnboardingIntentCreateReq) (res *v1.OnboardingIntentCreateRes, err error)
	TicketCreate(ctx context.Context, req *v1.TicketCreateReq) (res *v1.TicketCreateRes, err error)
	VerificationCodeCreate(ctx context.Context, req *v1.VerificationCodeCreateReq) (res *v1.VerificationCodeCreateRes, err error)
}
