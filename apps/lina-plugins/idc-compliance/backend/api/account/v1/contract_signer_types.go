package v1

type ContractSignerItem struct {
	SignerNo                    string `json:"signerNo" dc:"Opaque enterprise contract signer number" eg:"csgn_0123456789abcdef0123456789abcdef"`
	MaskedName                  string `json:"maskedName" dc:"Masked signer legal name" eg:"J*** D**"`
	MaskedIdentityNo            string `json:"maskedIdentityNo" dc:"Masked identity document number" eg:"110***********0019"`
	MaskedPhone                 string `json:"maskedPhone" dc:"Masked verified phone candidate" eg:"+86****1234"`
	IdentityStatusCode          string `json:"identityStatusCode" dc:"Identity state: unverified, verifying, verified, review_required, or expired" eg:"verified"`
	IdentityAttemptNo           string `json:"identityAttemptNo" dc:"Current identity verification attempt number" eg:"csia_0123456789abcdef0123456789abcdef"`
	IdentityVerificationVersion int64  `json:"identityVerificationVersion" dc:"Authoritative identity verification version" eg:"1"`
	IdentityVerifiedAt          int64  `json:"identityVerifiedAt" dc:"Authoritative identity verification time as Unix milliseconds" eg:"1786502460000"`
	PhoneStatusCode             string `json:"phoneStatusCode" dc:"Phone state: unverified, verifying, or verified" eg:"verified"`
	PhoneVerificationVersion    int64  `json:"phoneVerificationVersion" dc:"Notification security-code verification version" eg:"1"`
	PhoneVerifiedAt             int64  `json:"phoneVerifiedAt" dc:"Phone verification time as Unix milliseconds" eg:"1786502460000"`
	StatusCode                  string `json:"statusCode" dc:"Directory state: active or disabled" eg:"active"`
	AuthorizationVersion        int64  `json:"authorizationVersion" dc:"Enterprise signer authorization version" eg:"1"`
	Version                     int64  `json:"version" dc:"Optimistic signer version" eg:"2"`
	CreatedAt                   int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1786502400000"`
	UpdatedAt                   int64  `json:"updatedAt" dc:"Update time as Unix milliseconds" eg:"1786502460000"`
}

type ContractSignerIdentityStartItem struct {
	SignerNo        string `json:"signerNo"`
	AttemptNo       string `json:"attemptNo"`
	StatusCode      string `json:"statusCode"`
	ContinuationURL string `json:"continuationUrl"`
	ExpiresAt       int64  `json:"expiresAt"`
}

type ContractSignerHumanChallengeItem struct {
	ChallengeNo      string `json:"challengeNo"`
	ProviderCode     string `json:"providerCode"`
	ProviderAction   string `json:"providerAction"`
	Kind             string `json:"kind"`
	ImageDataURL     string `json:"imageDataUrl"`
	AccessiblePrompt string `json:"accessiblePrompt"`
	SiteKey          string `json:"siteKey"`
	ExpiresAt        int64  `json:"expiresAt"`
}

type ContractSignerHumanProofItem struct {
	ProofToken string `json:"proofToken"`
	ExpiresAt  int64  `json:"expiresAt"`
}
type ContractSignerPhoneCodeItem struct {
	VerificationNo    string `json:"verificationNo"`
	MaskedDestination string `json:"maskedDestination"`
	DeliveryState     string `json:"deliveryState"`
	ExpiresAt         int64  `json:"expiresAt"`
	RetryAfterAt      int64  `json:"retryAfterAt"`
}
