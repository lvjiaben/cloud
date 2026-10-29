// This file defines one authenticated login-contact code verification step.
package v1

import "github.com/gogf/gf/v2/frame/g"

// LoginContactCodeVerifyReq verifies a code bound to the current account and session.
type LoginContactCodeVerifyReq struct {
	g.Meta          `path:"/account/security/contact-codes/{verificationNo}/verification" method:"post" tags:"IDC Account Notifications" summary:"Verify a login contact code" dc:"Verifies one expiring code with a persistent attempt limit. This step does not change contact information; saving requires both old and new proofs."`
	VerificationNo  string `json:"verificationNo" v:"required|max-length:48" dc:"Opaque verification proof identifier" eg:"nver_example"`
	Kind            string `json:"kind" v:"required|in:email,phone" dc:"Contact to change: email or phone" eg:"email"`
	OldVerification string `json:"oldVerification" v:"max-length:48" dc:"Verified old-contact proof; empty when verifying the old contact" eg:"nver_old"`
	Destination     string `json:"destination" v:"max-length:320" dc:"New destination only; must match the code delivery request" eg:"new@example.com"`
	Locale          string `json:"locale" d:"zh-CN" v:"in:zh-CN,en-US" dc:"Message locale: zh-CN or en-US; defaults to zh-CN" eg:"zh-CN"`
	Code            string `json:"code" v:"required|regex:^[0-9]{6}$" dc:"Six-digit verification code received by the customer" eg:"123456"`
}

// LoginContactCodeVerifyRes confirms this proof is ready for the next step.
type LoginContactCodeVerifyRes struct{}
