package v1

import "github.com/gogf/gf/v2/frame/g"

type OnboardingHumanChallengeCreateReq struct {
	g.Meta           `path:"/public/account/onboarding/human-challenges" method:"post" tags:"IDC Customer Onboarding" summary:"Create a customer onboarding human challenge" dc:"Creates a purpose-bound, short-lived human-verification challenge before contact verification."`
	BrowserNonce     string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random browser binding value" eg:"browser_nonce_01"`
	Destination      string `json:"destination" v:"required|max-length:320" dc:"Email address or mobile number to verify" eg:"customer@example.com"`
	DestinationKind  string `json:"destinationKind" v:"required|in:email,sms" dc:"Verification destination kind: email or sms" eg:"email"`
	Locale           string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Requested message locale" eg:"zh-CN"`
	Username         string `json:"username" v:"required|length:2,64" dc:"Requested customer username bound to the proof" eg:"customer01"`
	CustomerType     string `json:"customerType" v:"required|in:individual" dc:"All new accounts start as individual customers" eg:"individual"`
	ReturnPath       string `json:"returnPath" v:"max-length:512" dc:"Optional safe customer application return path" eg:"/auth/register"`
	PresentationKind string `json:"presentationKind" v:"required|in:accessible,image" dc:"Challenge presentation: accessible or image" eg:"image"`
}
type OnboardingHumanChallengeCreateRes HumanChallengePresentation

type OnboardingHumanChallengeCompletionCreateReq struct {
	g.Meta          `path:"/public/account/onboarding/human-challenge-completions" method:"post" tags:"IDC Customer Onboarding" summary:"Complete a customer onboarding human challenge" dc:"Consumes one matching challenge and returns a short-lived, single-use proof."`
	BrowserNonce    string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random browser binding value" eg:"browser_nonce_01"`
	Destination     string `json:"destination" v:"required|max-length:320" dc:"Email address or mobile number bound to the challenge" eg:"customer@example.com"`
	DestinationKind string `json:"destinationKind" v:"required|in:email,sms" dc:"Verification destination kind: email or sms" eg:"email"`
	Locale          string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Requested message locale" eg:"zh-CN"`
	Username        string `json:"username" v:"required|length:2,64" dc:"Requested customer username bound to the challenge" eg:"customer01"`
	CustomerType    string `json:"customerType" v:"required|in:individual" dc:"All new accounts start as individual customers" eg:"individual"`
	ReturnPath      string `json:"returnPath" v:"max-length:512" dc:"Optional safe customer application return path" eg:"/auth/register"`
	ChallengeNo     string `json:"challengeNo" v:"required|max-length:96" dc:"Challenge number returned by challenge creation" eg:"CHL_01"`
	Answer          string `json:"answer" v:"max-length:256" dc:"Local image or accessible challenge answer" eg:"42"`
	ProviderToken   string `json:"providerToken" v:"max-length:4096" dc:"External provider token when that provider is configured" eg:"provider-proof-token"`
}
type OnboardingHumanChallengeCompletionCreateRes HumanChallengeProof

type OnboardingVerificationCodeCreateReq struct {
	g.Meta          `path:"/public/account/onboarding/verification-codes" method:"post" tags:"IDC Customer Onboarding" summary:"Send a customer onboarding verification code" dc:"Consumes a matching human proof and requests one rate-limited contact code delivery."`
	BrowserNonce    string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random browser binding value" eg:"browser_nonce_01"`
	Destination     string `json:"destination" v:"required|max-length:320" dc:"Email address or mobile number receiving the code" eg:"customer@example.com"`
	DestinationKind string `json:"destinationKind" v:"required|in:email,sms" dc:"Verification destination kind: email or sms" eg:"email"`
	Locale          string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Requested message locale" eg:"zh-CN"`
	Username        string `json:"username" v:"required|length:2,64" dc:"Requested customer username bound to the code" eg:"customer01"`
	CustomerType    string `json:"customerType" v:"required|in:individual" dc:"All new accounts start as individual customers" eg:"individual"`
	ReturnPath      string `json:"returnPath" v:"max-length:512" dc:"Optional safe customer application return path" eg:"/auth/register"`
	ProofToken      string `json:"proofToken" v:"required|max-length:4096" dc:"Short-lived single-use human-verification proof" eg:"human-proof-token"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Stable key preventing an unknown send result from being resent" eg:"send-code-0001"`
}
type OnboardingVerificationCodeCreateRes VerificationCodeProjection

type OnboardingIntentCreateReq struct {
	g.Meta          `path:"/public/account/onboarding/intents" method:"post" tags:"IDC Customer Onboarding" summary:"Verify the contact code and create a short-lived onboarding intent" dc:"Consumes the six-digit contact code and returns a short-lived intent bound to the verified request."`
	BrowserNonce    string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random browser binding value" eg:"browser_nonce_01"`
	Destination     string `json:"destination" v:"required|max-length:320" dc:"Verified email address or mobile number" eg:"customer@example.com"`
	DestinationKind string `json:"destinationKind" v:"required|in:email,sms" dc:"Verification destination kind: email or sms" eg:"email"`
	Locale          string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Requested message locale" eg:"zh-CN"`
	Username        string `json:"username" v:"required|length:2,64" dc:"Requested customer username bound to the intent" eg:"customer01"`
	CustomerType    string `json:"customerType" v:"required|in:individual" dc:"All new accounts start as individual customers" eg:"individual"`
	ReturnPath      string `json:"returnPath" v:"max-length:512" dc:"Optional safe customer application return path" eg:"/auth/register"`
	VerificationNo  string `json:"verificationNo" v:"required|max-length:48" dc:"Verification request number returned by code sending" eg:"VER_01"`
	Code            string `json:"code" v:"required|length:6,6" dc:"Six-digit single-use contact verification code" eg:"123456"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Stable key for idempotent intent creation" eg:"intent-0001"`
}

type OnboardingIntentProjection struct {
	IntentNo  string `json:"intentNo" dc:"Short-lived onboarding intent number" eg:"INT_01"`
	Token     string `json:"token" dc:"Opaque intent token returned only to the verified browser" eg:"intent-token"`
	ExpiresAt int64  `json:"expiresAt" dc:"Intent expiry as Unix timestamp in milliseconds" eg:"1787558400000"`
	Reused    bool   `json:"reused" dc:"Whether the idempotent request reused the existing intent" eg:"false"`
}
type OnboardingIntentCreateRes OnboardingIntentProjection
