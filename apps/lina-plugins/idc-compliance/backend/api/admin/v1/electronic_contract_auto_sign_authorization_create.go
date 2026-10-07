package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractAutoSignAuthorizationCreateReq enables one immutable authorization.
type ElectronicContractAutoSignAuthorizationCreateReq struct {
	g.Meta              `path:"/admin/compliance/electronic-contract/auto-sign-authorizations" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Enable enterprise auto-sign authorization" dc:"Enable one immutable installation-local enterprise auto-sign authorization after a recent full login, explicit confirmation, and current Tencent capability check." permission:"idc:admin:compliance:contract-auto-sign:manage"`
	CurrencyCode        string                                  `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Single ISO 4217 uppercase currency code" eg:"CNY"`
	AllowRequiredOrder  bool                                    `json:"allowRequiredOrder" dc:"Allow required order contracts" eg:"true"`
	AllowManualOptional bool                                    `json:"allowManualOptional" dc:"Allow administrator-created optional contracts" eg:"false"`
	MaxAmountMinor      string                                  `json:"maxAmountMinor" v:"required|max-length:19" dc:"Positive maximum amount for one contract in currency minor units" eg:"100000"`
	SuccessLimit        int64                                   `json:"successLimit" v:"required|min:1" dc:"Positive supplier-accepted start limit for this authorization lifetime" eg:"100"`
	ExpiresAt           int64                                   `json:"expiresAt" v:"required|min:1" dc:"Expiry as Unix timestamp in milliseconds; must be within one year" eg:"1798761600000"`
	Templates           []ElectronicContractAutoSignTemplateRef `json:"templates" v:"required" dc:"One to fifty exact enabled template versions; collection bounds are enforced by the authorization service" eg:"[]"`
	Confirmed           bool                                    `json:"confirmed" v:"required|in:true" dc:"True only after the administrator reviews the complete authorization summary" eg:"true"`
	IdempotencyKey      string                                  `json:"idempotencyKey" v:"required|length:8,128" dc:"Stable retry key; reusing it with changed input is rejected" eg:"contract-auto-sign-enable-20260816-001"`
}

type ElectronicContractAutoSignAuthorizationCreateRes struct {
	Item ElectronicContractAutoSignAuthorizationItem `json:"item" dc:"Created or safely replayed immutable authorization" eg:"{}"`
}
