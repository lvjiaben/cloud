package v1

// ElectronicContractAutoSignTemplateRef freezes one exact enabled template version.
type ElectronicContractAutoSignTemplateRef struct {
	TemplateNo string `json:"templateNo" dc:"Stable electronic contract template number" eg:"contract_template_0123456789abcdef"`
	VersionNo  int64  `json:"versionNo" dc:"Exact immutable template version number" eg:"3"`
	Name       string `json:"name" dc:"Template name snapshot for management display" eg:"Enterprise service agreement"`
}

// ElectronicContractAutoSignAuthorizationItem is one immutable or revoked authorization.
type ElectronicContractAutoSignAuthorizationItem struct {
	AuthorizationNo     string                                  `json:"authorizationNo" dc:"Stable enterprise auto-sign authorization number" eg:"casa_0123456789abcdef0123456789abcdef"`
	CurrencyCode        string                                  `json:"currencyCode" dc:"Single ISO 4217 uppercase currency code" eg:"CNY"`
	AllowRequiredOrder  bool                                    `json:"allowRequiredOrder" dc:"Whether required order contracts may use this authorization" eg:"true"`
	AllowManualOptional bool                                    `json:"allowManualOptional" dc:"Whether administrator-created optional contracts may use this authorization" eg:"false"`
	MaxAmountMinor      string                                  `json:"maxAmountMinor" dc:"Maximum total amount of one contract in currency minor units" eg:"100000"`
	SuccessLimit        int64                                   `json:"successLimit" dc:"Maximum supplier-accepted starts during this authorization lifetime" eg:"100"`
	SuccessCount        int64                                   `json:"successCount" dc:"Supplier-accepted starts already consumed" eg:"12"`
	StatusCode          string                                  `json:"statusCode" dc:"Authorization state: active, expired, or revoked" eg:"active"`
	ExpiresAt           int64                                   `json:"expiresAt" dc:"Authorization expiry as Unix timestamp in milliseconds" eg:"1798761600000"`
	CreatedByUserID     int                                     `json:"createdByUserId" dc:"Staff user that enabled the authorization" eg:"1"`
	RevokedByUserID     *int                                    `json:"revokedByUserId" dc:"Staff user that revoked the authorization when present" eg:"1"`
	RevokedAt           *int64                                  `json:"revokedAt" dc:"Revocation time as Unix timestamp in milliseconds when present" eg:"1790000000000"`
	Version             int64                                   `json:"version" dc:"Optimistic lock version used by revocation" eg:"1"`
	CreatedAt           int64                                   `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1767225600000"`
	Templates           []ElectronicContractAutoSignTemplateRef `json:"templates" dc:"Exact immutable template versions covered by this authorization" eg:"[]"`
}
