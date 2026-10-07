// This file defines manual optional electronic-contract preparation.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractCreateReq creates one non-blocking contract for an eligible order.
type ElectronicContractCreateReq struct {
	g.Meta         `path:"/admin/compliance/electronic-contracts" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Create an optional electronic contract" dc:"Prepare one immutable seven-day contract preview for an eligible order. The command never changes a no-contract order into a fulfillment gate and requires the independent contract-create permission." permission:"idc:admin:compliance:contract:create"`
	OrderNo        string `json:"orderNo" v:"required|max-length:40" dc:"Exact order number inside the caller's evaluated data scope" eg:"ORDER_20260815-003"`
	TemplateNo     string `json:"templateNo" v:"required|max-length:64" dc:"Exact enabled template matching the order's customer type" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	SignerNo       string `json:"signerNo" v:"max-length:64" dc:"Required verified enterprise signer candidate; must be empty for an individual customer" eg:"csgn_0123456789abcdef0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:16,128" dc:"Retry key bound to this exact order and template" eg:"contract-create-20260815-optional-0001"`
}

// ElectronicContractCreateRes returns the immutable pre-sign preview.
type ElectronicContractCreateRes struct {
	Item *ElectronicContractPreparationItem `json:"item" dc:"Created or safely replayed optional contract preview" eg:"{}"`
}

// ElectronicContractPreparationItem is the safe pre-sign management projection.
type ElectronicContractPreparationItem struct {
	ContractNo             string                                  `json:"contractNo" dc:"Installation-local opaque contract number" eg:"CTR_0123456789ABCDEF"`
	OrderNo                string                                  `json:"orderNo" dc:"Order bound to this one-contract snapshot" eg:"ORDER_20260815-003"`
	CustomerType           string                                  `json:"customerType" dc:"Verified individual or enterprise signer type" eg:"individual"`
	CustomerName           string                                  `json:"customerName" dc:"Verified legal customer name frozen into the preview" eg:"Example Customer"`
	SignerNo               string                                  `json:"signerNo" dc:"Frozen signer or handler number"`
	SignerName             string                                  `json:"signerName" dc:"Frozen personal signer or enterprise handler name"`
	SignerPhoneMasked      string                                  `json:"signerPhoneMasked" dc:"Masked frozen signer phone"`
	MerchantName           string                                  `json:"merchantName" dc:"Installation merchant legal name frozen into the preview" eg:"Example IDC Limited"`
	TemplateNo             string                                  `json:"templateNo" dc:"Exact immutable template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	TemplateVersion        int64                                   `json:"templateVersion" dc:"Exact immutable template version" eg:"5"`
	TemplateName           string                                  `json:"templateName" dc:"Template display name" eg:"Optional service contract"`
	LanguageCode           string                                  `json:"languageCode" dc:"Explicit zh-CN or en-US preview language" eg:"en-US"`
	SigningOrderCode       string                                  `json:"signingOrderCode" dc:"customer_first, merchant_first, or parallel" eg:"customer_first"`
	StatusCode             string                                  `json:"statusCode" dc:"Initial contract state, always pending_launch for this response" eg:"pending_launch"`
	TotalAmountMinor       string                                  `json:"totalAmountMinor" dc:"Exact total in minor units encoded as a decimal string" eg:"129900"`
	CurrencyCode           string                                  `json:"currencyCode" dc:"ISO 4217 order currency" eg:"CNY"`
	CreatedAt              int64                                   `json:"createdAt" dc:"Database-authoritative creation time as Unix milliseconds" eg:"1786782600000"`
	SigningExpiresAt       int64                                   `json:"signingExpiresAt" dc:"Fixed seven-day signing expiry as Unix milliseconds" eg:"1787387400000"`
	RequiredForFulfillment bool                                    `json:"requiredForFulfillment" dc:"Whether signing gates fulfillment; always false for this optional command" eg:"false"`
	SigningDate            string                                  `json:"signingDate" dc:"Blank until Tencent returns an authoritative completed signature" eg:""`
	SigningDateNoticeCode  string                                  `json:"signingDateNoticeCode" dc:"Stable notice explaining that the signing date is filled after authoritative completion" eg:"filled_after_authoritative_signature"`
	Lines                  []ElectronicContractPreparationLineItem `json:"lines" dc:"All order services rendered in the template language" eg:"[]"`
	Reused                 bool                                    `json:"reused" dc:"Whether the same immutable contract was safely replayed" eg:"false"`
	Version                int64                                   `json:"version" dc:"Current contract version used by the separate submit-signing command" eg:"1"`
}

// ElectronicContractPreparationLineItem is one frozen service line.
type ElectronicContractPreparationLineItem struct {
	OrderItemNo    string `json:"orderItemNo" dc:"Opaque order-line number" eg:"ITEM_001"`
	ServiceName    string `json:"serviceName" dc:"Authoritative service name in the template language" eg:"Dedicated Server"`
	Quantity       int    `json:"quantity" dc:"Purchased quantity in this line" eg:"1"`
	PurchaseCycles int    `json:"purchaseCycles" dc:"Number of purchased billing cycles" eg:"1"`
	PeriodUnitCode string `json:"periodUnitCode" dc:"Stable service-period unit" eg:"month"`
	PeriodCount    int    `json:"periodCount" dc:"Units in one purchased period" eg:"12"`
	AmountMinor    string `json:"amountMinor" dc:"Exact line amount in minor units encoded as a decimal string" eg:"99900"`
}
