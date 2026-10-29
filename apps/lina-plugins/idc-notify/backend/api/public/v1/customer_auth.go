// This file defines explicit customer login and purpose-bound contact recovery APIs.
package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerPasswordLoginReq defines the bounded customer authentication request.
type CustomerPasswordLoginReq struct {
	g.Meta   `path:"/public/account/password-logins" method:"post" tags:"IDC Customer Authentication" summary:"Authenticate a customer with an explicit login method" dc:"Uses public host authentication and plugin-owned contact verification without accessing host storage."`
	Kind     string `json:"kind" v:"required|in:username,email,phone" dc:"Independent username, current email or current phone method" eg:"email"`
	Account  string `json:"account" v:"required|max-length:320" dc:"Login identifier for the selected method" eg:"customer@example.com"`
	Password string `json:"password" v:"required|max-length:128" dc:"Account password, never stored by the plugin" eg:"example-password"`
}

// CustomerPasswordLoginRes returns the authentication outcome.
type CustomerPasswordLoginRes struct {
	AccessToken  string `json:"accessToken" dc:"Ordinary host access token" eg:"access-token"`
	RefreshToken string `json:"refreshToken" dc:"Ordinary host refresh token" eg:"refresh-token"`
}

// RecoveryHumanChallengeCreateReq defines the bounded customer authentication request.
type RecoveryHumanChallengeCreateReq struct {
	g.Meta `path:"/public/account/password-recovery/human-challenges" method:"post" tags:"IDC Customer Authentication" summary:"Create a password recovery human challenge" dc:"Uses public host authentication and plugin-owned contact verification without accessing host storage."`

	Kind             string `json:"kind" v:"required|in:email,phone" dc:"Contact method: email or phone" eg:"email"`
	Account          string `json:"account" v:"required|max-length:320" dc:"Current bound email address or mobile number" eg:"customer@example.com"`
	BrowserNonce     string `json:"browserNonce" v:"required|length:16,128" dc:"Random browser binding for this recovery attempt" eg:"recovery-browser-0001"`
	Locale           string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Verification message language" eg:"zh-CN"`
	PresentationKind string `json:"presentationKind" v:"required|in:accessible,image" dc:"Human challenge presentation" eg:"accessible"`
}

// RecoveryHumanChallengeCreateRes returns the bounded verification projection.
type RecoveryHumanChallengeCreateRes HumanChallengePresentation

// RecoveryHumanCompletionCreateReq defines the bounded customer authentication request.
type RecoveryHumanCompletionCreateReq struct {
	g.Meta `path:"/public/account/password-recovery/human-challenge-completions" method:"post" tags:"IDC Customer Authentication" summary:"Complete a password recovery human challenge" dc:"Uses public host authentication and plugin-owned contact verification without accessing host storage."`

	Kind          string `json:"kind" v:"required|in:email,phone" dc:"Contact method: email or phone" eg:"email"`
	Account       string `json:"account" v:"required|max-length:320" dc:"Current bound email address or mobile number" eg:"customer@example.com"`
	BrowserNonce  string `json:"browserNonce" v:"required|length:16,128" dc:"Random browser binding for this recovery attempt" eg:"recovery-browser-0001"`
	Locale        string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Verification message language" eg:"zh-CN"`
	ChallengeNo   string `json:"challengeNo" v:"required|max-length:96" dc:"Purpose-bound challenge number" eg:"CHL_01"`
	Answer        string `json:"answer" v:"max-length:256" dc:"Local human challenge answer" eg:"42"`
	ProviderToken string `json:"providerToken" v:"max-length:4096" dc:"Configured external human-verification token" eg:"provider-token"`
}

// RecoveryHumanCompletionCreateRes returns the bounded verification projection.
type RecoveryHumanCompletionCreateRes HumanChallengeProof

// RecoveryCodeCreateReq defines the bounded customer authentication request.
type RecoveryCodeCreateReq struct {
	g.Meta `path:"/public/account/password-recovery/verification-codes" method:"post" tags:"IDC Customer Authentication" summary:"Send a password recovery code" dc:"Uses public host authentication and plugin-owned contact verification without accessing host storage."`

	Kind           string `json:"kind" v:"required|in:email,phone" dc:"Contact method: email or phone" eg:"email"`
	Account        string `json:"account" v:"required|max-length:320" dc:"Current bound email address or mobile number" eg:"customer@example.com"`
	BrowserNonce   string `json:"browserNonce" v:"required|length:16,128" dc:"Random browser binding for this recovery attempt" eg:"recovery-browser-0001"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Verification message language" eg:"zh-CN"`
	ProofToken     string `json:"proofToken" v:"required|max-length:4096" dc:"Single-use human-verification proof" eg:"human-proof"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,64" dc:"Stable send request key" eg:"recovery-send-0001"`
}

// RecoveryCodeCreateRes returns the bounded verification projection.
type RecoveryCodeCreateRes VerificationCodeProjection

// RecoveryPasswordResetReq defines the bounded customer authentication request.
type RecoveryPasswordResetReq struct {
	g.Meta `path:"/public/account/password-recovery/password-resets" method:"post" tags:"IDC Customer Authentication" summary:"Reset a password after verifying the bound contact" dc:"Uses public host authentication and plugin-owned contact verification without accessing host storage."`

	Kind           string `json:"kind" v:"required|in:email,phone" dc:"Contact method: email or phone" eg:"email"`
	Account        string `json:"account" v:"required|max-length:320" dc:"Current bound email address or mobile number" eg:"customer@example.com"`
	BrowserNonce   string `json:"browserNonce" v:"required|length:16,128" dc:"Random browser binding for this recovery attempt" eg:"recovery-browser-0001"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Verification message language" eg:"zh-CN"`
	VerificationNo string `json:"verificationNo" v:"required|max-length:48" dc:"Contact verification request number" eg:"nver_01"`
	Code           string `json:"code" v:"required|length:6,6" dc:"Six-digit contact verification code" eg:"123456"`
	Password       string `json:"password" v:"required|length:8,32" dc:"New account password" eg:"new-example-password"`
}

// RecoveryPasswordResetRes returns the authentication outcome.
type RecoveryPasswordResetRes struct {
	Reset          bool   `json:"reset" dc:"Whether the password was saved" eg:"true"`
	CleanupToken   string `json:"cleanupToken" dc:"Opaque grant usable only to retry this reset session cleanup" eg:"opaque-cleanup-grant"`
	CleanupPending bool   `json:"cleanupPending" dc:"Whether revoking older sessions still needs a retry" eg:"false"`
}

// RecoverySessionCleanupReq retries only a completed reset's older sessions.
type RecoverySessionCleanupReq struct {
	g.Meta         `path:"/public/account/password-recovery/session-cleanups" method:"post" tags:"IDC Customer Authentication" summary:"Retry session cleanup for a completed password reset" dc:"Requires a separate short-lived cleanup grant and cannot change a password or return a login token."`
	Kind           string `json:"kind" v:"required|in:email,phone" dc:"Verified contact method" eg:"email"`
	Account        string `json:"account" v:"required|max-length:320" dc:"Current verified login contact" eg:"customer@example.com"`
	BrowserNonce   string `json:"browserNonce" v:"required|length:16,128" dc:"Original browser binding" eg:"recovery-browser-0001"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Recovery message language" eg:"zh-CN"`
	VerificationNo string `json:"verificationNo" v:"required|max-length:48" dc:"Completed reset verification number" eg:"nver_01"`
	CleanupToken   string `json:"cleanupToken" v:"required|length:64,64" dc:"Opaque cleanup-only grant returned after password saving" eg:"opaque-cleanup-grant"`
}

// RecoverySessionCleanupRes reports pending cleanup without repeating a reset.
type RecoverySessionCleanupRes struct {
	Pending bool `json:"pending" dc:"Whether cleanup still needs a retry" eg:"false"`
}
