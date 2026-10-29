// This file defines authenticated old/new login-contact code delivery.
package v1

import "github.com/gogf/gf/v2/frame/g"

// LoginContactCodeCreateReq sends an old-contact code or, after verifying it, a new-contact code.
type LoginContactCodeCreateReq struct {
	g.Meta          `path:"/account/security/contact-codes" method:"post" tags:"IDC Account Notifications" summary:"Send a login contact verification code" dc:"Sends a rate-limited code for the authenticated customer's old or new login contact. The old contact is read from the current identity, never from this request."`
	Kind            string `json:"kind" v:"required|in:email,phone" dc:"Contact to change: email or phone" eg:"email"`
	OldVerification string `json:"oldVerification" v:"max-length:48" dc:"Verified old-contact proof; empty for the first step" eg:"nver_example"`
	Destination     string `json:"destination" v:"max-length:320" dc:"New contact only; empty when sending the old-contact code" eg:"new@example.com"`
	Locale          string `json:"locale" d:"zh-CN" v:"in:zh-CN,en-US" dc:"Message locale: zh-CN or en-US; defaults to zh-CN" eg:"zh-CN"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|max-length:128" dc:"Unique send attempt key; replaying it never sends another message" eg:"contact-send-1700000000"`
}

// LoginContactCodeCreateRes contains delivery state and cooldown, never the code.
type LoginContactCodeCreateRes struct {
	VerificationNo    string `json:"verificationNo" dc:"Opaque verification proof identifier" eg:"nver_example"`
	MaskedDestination string `json:"maskedDestination" dc:"Masked destination selected by the server" eg:"al***@example.com"`
	DeliveryState     string `json:"deliveryState" dc:"Delivery state: prepared, accepted, delivered, result_unknown, or failed" eg:"accepted"`
	ExpiresAt         int64  `json:"expiresAt" dc:"Proof expiry as Unix timestamp in milliseconds" eg:"1700000600000"`
	RetryAfterAt      int64  `json:"retryAfterAt" dc:"Next allowed send as Unix timestamp in milliseconds" eg:"1700000060000"`
}
