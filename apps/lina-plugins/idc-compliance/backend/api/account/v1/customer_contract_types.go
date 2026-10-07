// This file defines customer-safe electronic contract response projections.

package v1

// CustomerContractItem is the customer-safe contract projection.
type CustomerContractItem struct {
	ContractNo       string `json:"contractNo" dc:"Opaque customer-owned contract number" eg:"ctr_0123456789abcdef0123456789abcdef"`
	OrderNo          string `json:"orderNo" dc:"Customer-owned order number" eg:"ord_0123456789abcdef0123456789abcdef"`
	TemplateName     string `json:"templateName" dc:"Frozen contract template name" eg:"Server service contract"`
	LanguageCode     string `json:"languageCode" dc:"Frozen contract language: zh-CN or en-US" eg:"zh-CN"`
	AmountMinor      string `json:"amountMinor" dc:"Frozen contract total as a decimal string in the currency minor unit" eg:"10000"`
	CurrencyCode     string `json:"currencyCode" dc:"Frozen ISO 4217 currency code" eg:"CNY"`
	StatusCode       string `json:"statusCode" dc:"Customer-safe state: processing, completed, or failed" eg:"processing"`
	CreatedAt        int64  `json:"createdAt" dc:"Contract creation time as Unix milliseconds" eg:"1786502400000"`
	SigningExpiresAt int64  `json:"signingExpiresAt" dc:"Fixed seven-day signing deadline as Unix milliseconds" eg:"1787107200000"`
	CanOpenSigning   bool   `json:"canOpenSigning" dc:"Whether a fresh 30-minute link may be generated for the existing flow" eg:"true"`
	CanRecheck       bool   `json:"canRecheck" dc:"Whether an authoritative Tencent read-only recheck is available" eg:"true"`
	Version          int64  `json:"version" dc:"Current local contract version" eg:"2"`
}

// CustomerSigningLinkItem is one non-persisted short-lived Tencent H5 link.
type CustomerSigningLinkItem struct {
	Contract  CustomerContractItem `json:"contract" dc:"Current customer-safe contract state associated with this link" eg:"{}"`
	URL       string               `json:"url" dc:"Fresh Tencent HTTPS signing URL" eg:"https://ess.tencentcloudapi.com/sign/example"`
	ExpiresAt int64                `json:"expiresAt" dc:"Thirty-minute link expiry as Unix milliseconds" eg:"1786504200000"`
}
