// Package v1 defines the public anonymous presales notification contract.
package v1

// HumanChallengePresentation is a safe, browser-renderable challenge projection.
type HumanChallengePresentation struct {
	ChallengeNo      string `json:"challengeNo" dc:"Opaque human-challenge number" eg:"hc_01JZ..."`
	ProviderCode     string `json:"providerCode" dc:"Human-verification provider code" eg:"builtin"`
	ProviderAction   string `json:"providerAction,omitempty" dc:"Provider action bound to this challenge" eg:"support.presales"`
	Kind             string `json:"kind" dc:"Challenge presentation kind" eg:"accessible"`
	ImageDataURL     string `json:"imageDataUrl,omitempty" dc:"Safe inline PNG data URL when the image presentation is selected" eg:"data:image/png;base64,..."`
	AccessiblePrompt string `json:"accessiblePrompt,omitempty" dc:"Accessible challenge prompt" eg:"Enter the result of 3 plus 4"`
	SiteKey          string `json:"siteKey,omitempty" dc:"Public Turnstile site key when configured" eg:"0x4AAAA..."`
	ExpiresAt        int64  `json:"expiresAt" dc:"Challenge expiry as a Unix timestamp in milliseconds" eg:"1786752300000"`
}

// HumanChallengeProof is a short-lived, single-use proof.
type HumanChallengeProof struct {
	ProofToken string `json:"proofToken" dc:"Short-lived proof token bound to this browser and operation" eg:"proof_01JZ..."`
	ExpiresAt  int64  `json:"expiresAt" dc:"Proof expiry as a Unix timestamp in milliseconds" eg:"1786752300000"`
}

// VerificationCodeProjection contains masked delivery evidence only.
type VerificationCodeProjection struct {
	VerificationNo    string `json:"verificationNo" dc:"Opaque verification number" eg:"nver_01JZ..."`
	MaskedDestination string `json:"maskedDestination" dc:"Masked one-time delivery destination" eg:"j***@example.com"`
	DeliveryState     string `json:"deliveryState" dc:"Observed provider delivery state" eg:"accepted"`
	ExpiresAt         int64  `json:"expiresAt" dc:"Code expiry as a Unix timestamp in milliseconds" eg:"1786752300000"`
	RetryAfterAt      int64  `json:"retryAfterAt" dc:"Earliest time another qualified code may be requested, as a Unix timestamp in milliseconds" eg:"1786751760000"`
}

// PresalesAccessProjection carries the browser's current two-hour access token.
type PresalesAccessProjection struct {
	TicketNo          string `json:"ticketNo" dc:"Created or renewed anonymous presales ticket number" eg:"TKT_01JZ..."`
	AccessToken       string `json:"accessToken" dc:"Current-browser anonymous ticket access token; never persisted by notify" eg:"jK9x..."`
	ExpiresAt         int64  `json:"expiresAt" dc:"Access expiry as a Unix timestamp in milliseconds" eg:"1786758300000"`
	LinkDeliveryState string `json:"linkDeliveryState" dc:"Observed access-link delivery state" eg:"accepted"`
}
