package public

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func onboardingBinding(reqBrowser, destination, destinationKind, locale, username, customerType, returnPath string) notification.OnboardingBindingInput {
	return notification.OnboardingBindingInput{BrowserNonce: []byte(reqBrowser), Destination: []byte(destination), DestinationKind: destinationKind, Locale: locale, Username: username, CustomerType: customerType, ReturnPath: returnPath}
}

func (c *ControllerV1) OnboardingHumanChallengeCreate(ctx context.Context, req *v1.OnboardingHumanChallengeCreateReq) (*v1.OnboardingHumanChallengeCreateRes, error) {
	binding := onboardingBinding(req.BrowserNonce, req.Destination, req.DestinationKind, req.Locale, req.Username, req.CustomerType, req.ReturnPath)
	req.BrowserNonce, req.Destination = "", ""
	item, err := c.notifications.CreateOnboardingHumanChallenge(ctx, notification.OnboardingHumanChallengeInput{OnboardingBindingInput: binding, PresentationKind: req.PresentationKind})
	if err != nil {
		return nil, err
	}
	return (*v1.OnboardingHumanChallengeCreateRes)(humanChallengeProjection(item)), nil
}

func (c *ControllerV1) OnboardingHumanChallengeCompletionCreate(ctx context.Context, req *v1.OnboardingHumanChallengeCompletionCreateReq) (*v1.OnboardingHumanChallengeCompletionCreateRes, error) {
	binding := onboardingBinding(req.BrowserNonce, req.Destination, req.DestinationKind, req.Locale, req.Username, req.CustomerType, req.ReturnPath)
	answer, providerToken := []byte(req.Answer), []byte(req.ProviderToken)
	req.BrowserNonce, req.Destination, req.Answer, req.ProviderToken = "", "", "", ""
	item, err := c.notifications.CompleteOnboardingHumanChallenge(ctx, notification.OnboardingHumanCompletionInput{OnboardingBindingInput: binding, ChallengeNo: req.ChallengeNo, Answer: answer, ProviderToken: providerToken})
	if err != nil {
		return nil, err
	}
	return (*v1.OnboardingHumanChallengeCompletionCreateRes)(humanProofProjection(item)), nil
}

func (c *ControllerV1) OnboardingVerificationCodeCreate(ctx context.Context, req *v1.OnboardingVerificationCodeCreateReq) (*v1.OnboardingVerificationCodeCreateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, notificationUnavailable()
	}
	binding := onboardingBinding(req.BrowserNonce, req.Destination, req.DestinationKind, req.Locale, req.Username, req.CustomerType, req.ReturnPath)
	req.BrowserNonce, req.Destination = "", ""
	item, err := c.notifications.SendOnboardingVerificationCode(ctx, notification.OnboardingVerificationInput{OnboardingBindingInput: binding, ProofToken: req.ProofToken, IdempotencyKey: req.IdempotencyKey, ClientIP: []byte(request.GetClientIp())})
	if err != nil {
		return nil, err
	}
	return (*v1.OnboardingVerificationCodeCreateRes)(verificationProjection(item)), nil
}

func (c *ControllerV1) OnboardingIntentCreate(ctx context.Context, req *v1.OnboardingIntentCreateReq) (*v1.OnboardingIntentCreateRes, error) {
	binding := onboardingBinding(req.BrowserNonce, req.Destination, req.DestinationKind, req.Locale, req.Username, req.CustomerType, req.ReturnPath)
	code := []byte(req.Code)
	req.BrowserNonce, req.Destination, req.Code = "", "", ""
	item, err := c.notifications.CreateVerifiedOnboardingIntent(ctx, notification.OnboardingIntentInput{OnboardingBindingInput: binding, VerificationNo: req.VerificationNo, Code: code, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.OnboardingIntentCreateRes{IntentNo: item.IntentNo, Token: item.Token, ExpiresAt: item.ExpiresAt.UTC().UnixMilli(), Reused: item.Reused}, nil
}
