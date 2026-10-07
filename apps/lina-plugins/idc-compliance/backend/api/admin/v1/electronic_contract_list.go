// This file defines the bounded management electronic-contract directory API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractListReq reads one bounded page with its authorized total.
type ElectronicContractListReq struct {
	Page       int `json:"page" v:"min:0|max:1000000" dc:"Requested page number; omit for cursor pagination" eg:"1"`
	g.Meta     `path:"/admin/compliance/electronic-contracts" method:"get" tags:"IDC Compliance Electronic Contract" summary:"List electronic contracts" dc:"Return one bounded merchant-local page with an authorized total and safe contract, template, amount, lifecycle, and expiry fields. The response never contains supplier identifiers, private storage references, signing links, or decrypted contract bodies." permission:"idc:admin:compliance:contract:query"`
	StatusCode string `json:"statusCode" v:"max-length:24" dc:"Optional exact state: draft, pending_launch, signing, completed, rejected, expired, revoked, failed, reissuing, or terminated; empty includes all states" eg:"signing"`
	OrderNo    string `json:"orderNo" v:"max-length:64" dc:"Optional exact opaque order number; empty includes all visible orders" eg:"ORDER_20260815-003"`
	CustomerNo string `json:"customerNo" v:"max-length:64" dc:"Optional exact opaque customer number; empty includes all visible customers" eg:"CUS_0123456789abcdef0123456789abcdef"`
	PageSize   int    `json:"pageSize" v:"min:0|max:100" dc:"Page size; zero selects 20 and the maximum is 100" eg:"20"`
	Cursor     string `json:"cursor" v:"max-length:256" dc:"Opaque next-page cursor from the preceding response; empty reads the first page" eg:"eyJjcmVhdGVkQXQiOiIyMDI2LTA4LTE4VDA4OjAwOjAwWiIsImlkIjoxfQ"`
}

// ElectronicContractListRes returns one bounded page with an exact authorized total.
type ElectronicContractListRes struct {
	Total      int                          `json:"total" dc:"Total matching records in the authorized scope" eg:"25"`
	Items      []ElectronicContractListItem `json:"items" dc:"Secret-safe electronic-contract rows" eg:"[]"`
	NextCursor string                       `json:"nextCursor" dc:"Opaque next-page cursor; empty on the final page" eg:"eyJjcmVhdGVkQXQiOiIyMDI2LTA4LTE4VDA4OjAwOjAwWiIsImlkIjoxfQ"`
}

// ElectronicContractListItem is the safe management projection shared by list and detail.
type ElectronicContractListItem struct {
	ContractNo             string `json:"contractNo" dc:"Installation-local opaque contract number" eg:"CTR_0123456789ABCDEF"`
	CustomerNo             string `json:"customerNo" dc:"Opaque customer owner number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	OrderNo                string `json:"orderNo" dc:"Opaque order number frozen into this contract" eg:"ORDER_20260815-003"`
	ContractTypeCode       string `json:"contractTypeCode" dc:"Stable contract source type" eg:"order_service"`
	RequiredForFulfillment bool   `json:"requiredForFulfillment" dc:"Whether completed signing gates order fulfillment" eg:"false"`
	TemplateNo             string `json:"templateNo" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	TemplateVersion        int64  `json:"templateVersion" dc:"Immutable template version used by this contract" eg:"5"`
	TemplateName           string `json:"templateName" dc:"Frozen template display name" eg:"Optional service contract"`
	LanguageCode           string `json:"languageCode" dc:"Contract language: zh-CN or en-US" eg:"zh-CN"`
	SubjectTypeCode        string `json:"subjectTypeCode" dc:"Signer subject type: individual or enterprise" eg:"enterprise"`
	SigningOrderCode       string `json:"signingOrderCode" dc:"Frozen signing order: customer_first, merchant_first, or parallel" eg:"customer_first"`
	AmountMinor            string `json:"amountMinor" dc:"Exact contract amount in minor units encoded as a decimal string" eg:"129900"`
	CurrencyCode           string `json:"currencyCode" dc:"ISO 4217 contract currency" eg:"CNY"`
	StatusCode             string `json:"statusCode" dc:"Contract state: draft, pending_launch, signing, completed, rejected, expired, revoked, failed, reissuing, or terminated" eg:"signing"`
	LaunchedAt             int64  `json:"launchedAt" dc:"Provider launch time as Unix timestamp in milliseconds; zero before launch" eg:"1787040000000"`
	CompletedAt            int64  `json:"completedAt" dc:"Authoritative completion time as Unix timestamp in milliseconds; zero until completed" eg:"1787043600000"`
	TerminatedAt           int64  `json:"terminatedAt" dc:"Authoritative termination time as Unix timestamp in milliseconds; zero until terminated" eg:"1787126400000"`
	SigningExpiresAt       int64  `json:"signingExpiresAt" dc:"Current fixed signing deadline as Unix timestamp in milliseconds; zero when not applicable" eg:"1787644800000"`
	CreatedAt              int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1787040000000"`
	UpdatedAt              int64  `json:"updatedAt" dc:"Last lifecycle update time as Unix timestamp in milliseconds" eg:"1787043600000"`
	Version                int64  `json:"version" dc:"Current optimistic contract version" eg:"3"`
}
