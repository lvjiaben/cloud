package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerIPChangeQuote struct {
	QuoteNo      string `json:"quoteNo" dc:"Immutable IP-change quote number" eg:"IPQ_0123456789abcdef"`
	ServiceNo    string `json:"serviceNo" dc:"Owned server service number" eg:"SVC_SERVER_0001"`
	CurrencyCode string `json:"currencyCode" dc:"Quote currency" eg:"CNY"`
	IPQuantity   int    `json:"ipQuantity" dc:"Current IP quantity that will be preserved" eg:"1"`
	FreeChange   bool   `json:"freeChange" dc:"Whether the published free allowance covers this request" eg:"true"`
	AmountMinor  int64  `json:"amountMinor" dc:"Final tax-inclusive amount in minor units" eg:"0"`
	ExpiresAt    int64  `json:"expiresAt" dc:"Quote and stock reservation expiry in Unix milliseconds" eg:"1787184900000"`
}

type ServerIPChangeQuoteCreateReq struct {
	g.Meta         `path:"/account/servers/{serviceNo}/ip-change-quotes" method:"post" tags:"IDC Customer Servers" summary:"Preview my server IP change" dc:"Validate the current customer's server, merchant rule, frequency and fresh local inventory, then create one immutable quote bound to a resource-owner reservation."`
	ServiceNo      string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Owned server service number" eg:"SVC_SERVER_0001"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Client-generated key reused only for this exact preview" eg:"ip-change-550e8400-e29b-41d4-a716-446655440000"`
}

type ServerIPChangeQuoteCreateRes struct {
	Quote *ServerIPChangeQuote `json:"quote" dc:"Current immutable quote and stock hold" eg:"{}"`
}

type ServerIPChangeCheckoutCreateReq struct {
	g.Meta         `path:"/account/server-ip-change-quotes/{quoteNo}/checkout" method:"post" tags:"IDC Customer Servers" summary:"Checkout my server IP change" dc:"Create a free executable IP-change operation or an ordinary payable order and invoice; no Provider task is created before financial settlement."`
	QuoteNo        string `json:"quoteNo" in:"path" v:"required|max-length:40" dc:"Immutable IP-change quote number" eg:"IPQ_0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Client-generated key reused only for this exact checkout" eg:"ip-checkout-550e8400-e29b-41d4-a716-446655440000"`
	ProofToken     string `json:"proofToken" v:"max-length:4096" dc:"Single-use current-session confirmation proof when merchant policy requires it" eg:"eyJ..."`
}

type ServerIPChangeCheckout struct {
	OperationNo     string `json:"operationNo" dc:"IP-change business operation number" eg:"IPO_0123456789abcdef"`
	OrderNo         string `json:"orderNo" dc:"Payable order number, or empty for a free change" eg:"IOR_0123456789abcdef"`
	InvoiceNo       string `json:"invoiceNo" dc:"Payable invoice number, or empty for a free change" eg:"IIN_0123456789abcdef"`
	ServiceNo       string `json:"serviceNo" dc:"Owned server service number" eg:"SVC_SERVER_0001"`
	Status          string `json:"status" dc:"ready or awaiting_financial_settlement" eg:"ready"`
	AmountMinor     int64  `json:"amountMinor" dc:"Final tax-inclusive amount in minor units" eg:"0"`
	CurrencyCode    string `json:"currencyCode" dc:"Checkout currency" eg:"CNY"`
	PaymentDueAt    int64  `json:"paymentDueAt" dc:"Payment and reservation deadline in Unix milliseconds" eg:"1787184900000"`
	PaymentRequired bool   `json:"paymentRequired" dc:"Whether financial settlement is required before execution" eg:"false"`
}

type ServerIPChangeCheckoutCreateRes struct {
	Checkout *ServerIPChangeCheckout `json:"checkout" dc:"Created or idempotently reused checkout" eg:"{}"`
}
