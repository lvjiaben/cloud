// This file exposes current-customer renewal quotations and confirmation through
// the existing renewal owner. Prices, ownership and expiry remain server owned.
package v1

import "github.com/gogf/gf/v2/frame/g"

// EntitlementRenewalQuoteReq quotes ordinary renewal of an owned long-term service.
type EntitlementRenewalQuoteReq struct {
	g.Meta         `path:"/account/entitlements/{entitlementNo}/renewal-quotes" method:"post" tags:"IDC Customer Billing" summary:"Quote my service renewal" dc:"Quote the current price for an owned service, preserving its existing specifications and checking the current sale state."`
	EntitlementNo  string `json:"entitlementNo" in:"path" v:"required|max-length:40" dc:"Owned service entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	RenewalCycles  int    `json:"renewalCycles" v:"required|between:1,120" dc:"Number of the service billing cycles to renew" eg:"1"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:16,128" dc:"Stable key retained when retrying this quote" eg:"renewal-550e8400-e29b-41d4-a716-446655440000"`
}

// EntitlementRenewalQuote is an immutable current-price preview, not a payment.
type EntitlementRenewalQuote struct {
	QuoteNo            string `json:"quoteNo" dc:"Owned immutable renewal quotation" eg:"RQ_0123456789abcdef0123456789abcdef"`
	CurrencyCode       string `json:"currencyCode" dc:"Exact service currency" eg:"CNY"`
	TotalAmountMinor   string `json:"totalAmountMinor" dc:"Exact payable amount in minor currency units" eg:"1000"`
	TaxAmountMinor     string `json:"taxAmountMinor" dc:"Included tax amount in minor currency units" eg:"0"`
	ProjectedExpiresAt int64  `json:"projectedExpiresAt" dc:"Expiry after successful settlement in Unix milliseconds" eg:"1795060800000"`
	ExpiresAt          int64  `json:"expiresAt" dc:"Quotation expiry in Unix milliseconds" eg:"1792382700000"`
}

type EntitlementRenewalQuoteRes struct {
	Quote *EntitlementRenewalQuote `json:"quote" dc:"Server-owned current renewal quotation" eg:"{}"`
}

// EntitlementRenewalConfirmReq creates the invoice for one still-current quote.
type EntitlementRenewalConfirmReq struct {
	g.Meta         `path:"/account/renewal-quotes/{quoteNo}/confirmations" method:"post" tags:"IDC Customer Billing" summary:"Confirm my service renewal" dc:"Confirm one owned current quotation and create its ordinary receivable invoice. Confirmation alone never extends a paid service."`
	QuoteNo        string `json:"quoteNo" in:"path" v:"required|max-length:40" dc:"Owned renewal quotation number" eg:"RQ_0123456789abcdef0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:16,128" dc:"Stable key retained when retrying this confirmation" eg:"renewal-confirm-550e8400-e29b-41d4-a716-446655440000"`
}

type EntitlementRenewalConfirmRes struct {
	Operation *TrialConversionOperation `json:"operation" dc:"Owned renewal invoice and settlement state" eg:"{}"`
}
