// This file defines customer-owned trial conversion and balance payment APIs.
// Ownership, prices and payment amounts are always resolved server-side.

package v1

import "github.com/gogf/gf/v2/frame/g"

// TrialConversionListReq is the customer-owned trial conversion contract.
type TrialConversionListReq struct {
	g.Meta   `path:"/account/trial-conversions" method:"get" tags:"IDC Customer Trials" summary:"List my trial services" dc:"Return one owned trial page with current formal billing periods." pageBridgeCapability:"trial-conversions.customer.v1" pageBridgeHost:"customer"`
	Page     int `json:"page" d:"1" v:"between:1,100000" dc:"One-based page number" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"between:1,100" dc:"Page size, at most 100" eg:"20"`
}

// TrialConversionPeriod is the customer-owned trial conversion contract.
type TrialConversionPeriod struct {
	BillingRuleNo  string `json:"billingRuleNo"  dc:"Current formal billing rule number" eg:"BIL_month"`
	PeriodUnitCode string `json:"periodUnitCode"  dc:"Billing unit: hour, day, month, or year" eg:"month"`
	PeriodCount    int    `json:"periodCount"  dc:"Number of units per billing cycle" eg:"1"`
}

// TrialConversionItem is the customer-owned trial conversion contract.
type TrialConversionItem struct {
	EntitlementNo   string                  `json:"entitlementNo"  dc:"Owned service entitlement number" eg:"ENT_demo"`
	ProductNameZhCN string                  `json:"productNameZhCN"  dc:"Frozen Simplified Chinese product name" eg:"Cloud server"`
	ProductNameEnUS string                  `json:"productNameEnUS"  dc:"Frozen English product name" eg:"Cloud server"`
	CurrencyCode    string                  `json:"currencyCode"  dc:"Exact service billing currency" eg:"CNY"`
	StatusCode      string                  `json:"statusCode"  dc:"Trial state: active or suspended" eg:"suspended"`
	ExpiresAt       int64                   `json:"expiresAt"  dc:"Trial expiry as Unix timestamp in milliseconds" eg:"1792000000000"`
	Periods         []TrialConversionPeriod `json:"periods"  dc:"Current formal periods; empty means contact support" eg:"[]"`
}

// TrialConversionListRes is the customer-owned trial conversion contract.
type TrialConversionListRes struct {
	Items []TrialConversionItem `json:"items"  dc:"Current owned trial page" eg:"[]"`
	Total int                   `json:"total"  dc:"Exact number of owned trial services" eg:"1"`
}

// TrialConversionQuoteCreateReq is the customer-owned trial conversion contract.
type TrialConversionQuoteCreateReq struct {
	g.Meta         `path:"/account/trial-conversions/{entitlementNo}/quotes" method:"post" tags:"IDC Customer Trials" summary:"Quote trial conversion" dc:"Quote a selected formal period at the current price without installation fees or payment." pageBridgeCapability:"trial-conversions.customer.v1" pageBridgeHost:"customer"`
	EntitlementNo  string `json:"entitlementNo" in:"path" v:"required|max-length:40" dc:"Owned service entitlement number" eg:"ENT_demo"`
	BillingRuleNo  string `json:"billingRuleNo" v:"required|max-length:40" dc:"Current formal billing rule number" eg:"BIL_month"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|max-length:128" dc:"Stable key reused when retrying this request" eg:"conversion-request-001"`
}

// TrialConversionQuote is the customer-owned trial conversion contract.
type TrialConversionQuote struct {
	QuoteNo          string `json:"quoteNo"  dc:"Immutable conversion quote number" eg:"QTE_demo"`
	CurrencyCode     string `json:"currencyCode"  dc:"Exact service billing currency" eg:"CNY"`
	NetAmountMinor   string `json:"netAmountMinor"  dc:"Tax-exclusive amount in minor units" eg:"10000"`
	TaxAmountMinor   string `json:"taxAmountMinor"  dc:"Tax amount in minor units" eg:"0"`
	TotalAmountMinor string `json:"totalAmountMinor"  dc:"Payable amount in minor units" eg:"10000"`
	ExpiresAt        int64  `json:"expiresAt"  dc:"Quote expiry as Unix timestamp in milliseconds" eg:"1792000000000"`
}

// TrialConversionQuoteCreateRes is the customer-owned trial conversion contract.
type TrialConversionQuoteCreateRes struct {
	Quote *TrialConversionQuote `json:"quote"  dc:"Frozen conversion quotation" eg:"{}"`
}

// TrialConversionConfirmReq is the customer-owned trial conversion contract.
type TrialConversionConfirmReq struct {
	g.Meta         `path:"/account/trial-conversion-quotes/{quoteNo}/confirmations" method:"post" tags:"IDC Customer Trials" summary:"Confirm trial conversion" dc:"Create one owned invoice; zero-amount conversions settle without deducting balance." pageBridgeCapability:"trial-conversions.customer.v1" pageBridgeHost:"customer"`
	QuoteNo        string `json:"quoteNo" in:"path" v:"required|max-length:40" dc:"Immutable conversion quote number" eg:"QTE_demo"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|max-length:128" dc:"Stable key reused when retrying this request" eg:"conversion-confirm-001"`
}

// TrialConversionOperation is the customer-owned trial conversion contract.
type TrialConversionOperation struct {
	OperationNo string `json:"operationNo"  dc:"Owned conversion operation number" eg:"ROP_demo"`
	InvoiceNo   string `json:"invoiceNo"  dc:"Owned conversion invoice number" eg:"INV_demo"`
	StatusCode  string `json:"statusCode"  dc:"Payment state: awaiting_financial_settlement or succeeded" eg:"awaiting_financial_settlement"`
}

// TrialConversionConfirmRes is the customer-owned trial conversion contract.
type TrialConversionConfirmRes struct {
	Operation *TrialConversionOperation `json:"operation"  dc:"Owned conversion payment intent" eg:"{}"`
}

// TrialConversionPayReq is the customer-owned trial conversion contract.
type TrialConversionPayReq struct {
	g.Meta      `path:"/account/trial-conversion-operations/{operationNo}/balance-payments" method:"post" tags:"IDC Customer Trials" summary:"Pay trial conversion from balance" dc:"Atomically debit same-currency balance and queue recovery of an already suspended trial." pageBridgeCapability:"trial-conversions.customer.v1" pageBridgeHost:"customer"`
	OperationNo string `json:"operationNo" in:"path" v:"required|max-length:40" dc:"Owned conversion operation number" eg:"ROP_demo"`
}

// TrialConversionPayRes is the customer-owned trial conversion contract.
type TrialConversionPayRes struct {
	Operation *TrialConversionOperation `json:"operation"  dc:"Owned conversion payment intent" eg:"{}"`
}
