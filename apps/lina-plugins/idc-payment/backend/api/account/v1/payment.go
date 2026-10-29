package v1

import "github.com/gogf/gf/v2/frame/g"

type PaymentChannelListReq struct {
	g.Meta       `path:"/account/payment-channels" method:"get" tags:"IDC Account Payments" summary:"List available customer payment channels" dc:"Return at most one hundred enabled channels and their verified modes for the optional exact currency. Credentials, merchant identifiers, and endpoints are never returned."`
	CurrencyCode string `json:"currencyCode" in:"query" v:"length:0,3" dc:"Optional exact three-letter uppercase currency" eg:"CNY"`
}
type PaymentChannelItem struct {
	ChannelNo   string   `json:"channelNo"`
	DisplayName string   `json:"displayName"`
	Provider    string   `json:"provider"`
	Modes       []string `json:"modes"`
	Currencies  []string `json:"currencies"`
}
type PaymentChannelListRes struct {
	Channels []*PaymentChannelItem `json:"channels"`
}

type PaymentCreateReq struct {
	g.Meta             `path:"/account/payments" method:"post" tags:"IDC Account Payments" summary:"Create a customer payment attempt" dc:"Re-read the current customer's payment preparation, optionally lock same-currency balance, and create one verified channel redirect. Customer, order, invoice, currency, and total amount are never accepted from the browser."`
	PaymentContextNo   string `json:"paymentContextNo" v:"required|max-length:40"`
	ChannelNo          string `json:"channelNo" v:"required|max-length:64"`
	ModeCode           string `json:"modeCode" v:"required|max-length:32"`
	BalanceAmountMinor string `json:"balanceAmountMinor" v:"required|max-length:20" dc:"Non-negative integer minor-unit balance amount; use 0 for channel-only payment" eg:"0"`
	OpenID             string `json:"openId" v:"max-length:128" dc:"Current official-account identity required only for an enabled WeChat JSAPI mode"`
}
type PaymentCreateRes struct {
	AttemptNo      string            `json:"attemptNo"`
	StatusCode     string            `json:"statusCode"`
	RedirectMethod string            `json:"redirectMethod"`
	RedirectURL    string            `json:"redirectUrl"`
	RedirectFields map[string]string `json:"redirectFields"`
	ExpiresAt      int64             `json:"expiresAt" dc:"Unix timestamp in milliseconds"`
}

type CreditRepaymentCreateReq struct {
	g.Meta      `path:"/account/credit/statements/{statementNo}/payments" method:"post" tags:"IDC Account Payments" summary:"Create an online credit-statement repayment" dc:"Re-read the current customer's credit statement and create one verified same-currency channel payment. The customer, currency, outstanding amount, and repayment result are controlled by the server."`
	StatementNo string `json:"statementNo" in:"path" v:"required|max-length:40"`
	ChannelNo   string `json:"channelNo" v:"required|max-length:64"`
	ModeCode    string `json:"modeCode" v:"required|max-length:32"`
	AmountMinor string `json:"amountMinor" v:"required|max-length:20" dc:"Positive partial or full repayment amount in integer minor units" eg:"10000"`
	OpenID      string `json:"openId" v:"max-length:128" dc:"Current official-account identity required only for an enabled WeChat JSAPI mode"`
}

type CreditRepaymentCreateRes = PaymentCreateRes

type BalanceRechargeCreateReq struct {
	g.Meta       `path:"/account/balance-recharges" method:"post" tags:"IDC Account Payments" summary:"Create a customer balance recharge" dc:"Create a verified channel payment for the current customer's same-currency balance. The credited customer and final balance posting are controlled by the core billing owner."`
	ChannelNo    string `json:"channelNo" v:"required|max-length:64"`
	ModeCode     string `json:"modeCode" v:"required|max-length:32"`
	CurrencyCode string `json:"currencyCode" v:"required|length:3,3" dc:"Exact three-letter uppercase recharge currency" eg:"CNY"`
	AmountMinor  string `json:"amountMinor" v:"required|max-length:20" dc:"Positive recharge amount in integer minor units" eg:"10000"`
	OpenID       string `json:"openId" v:"max-length:128" dc:"Current official-account identity required only for an enabled WeChat JSAPI mode"`
}

type BalanceRechargeCreateRes = PaymentCreateRes

type PaymentStatusReq struct {
	g.Meta    `path:"/account/payments/{attemptNo}" method:"get" tags:"IDC Account Payments" summary:"Read the current customer's payment confirmation state" dc:"Return only the local confirmation state. A browser return never marks payment successful; a verified active-query job may be queued when the channel supports it."`
	AttemptNo string `json:"attemptNo" in:"path" v:"required|max-length:40"`
}
type PaymentStatusRes struct {
	InvoiceNo          string `json:"invoiceNo,omitempty" dc:"Customer-owned invoice associated with this payment, omitted for balance recharge or credit repayment" eg:"INV_0123456789abcdef0123456789abcdef"`
	OrderNo            string `json:"orderNo,omitempty" dc:"Customer-owned order associated with this payment, omitted for balance recharge or credit repayment" eg:"ORD_0123456789abcdef0123456789abcdef"`
	AttemptNo          string `json:"attemptNo"`
	StatusCode         string `json:"statusCode"`
	ChannelNo          string `json:"channelNo"`
	ModeCode           string `json:"modeCode"`
	CurrencyCode       string `json:"currencyCode"`
	PurposeCode        string `json:"purposeCode"`
	PurposeReference   string `json:"purposeReference"`
	TotalAmountMinor   string `json:"totalAmountMinor"`
	ChannelAmountMinor string `json:"channelAmountMinor"`
	BalanceAmountMinor string `json:"balanceAmountMinor"`
	ExpiresAt          int64  `json:"expiresAt"`
	ConfirmedAt        *int64 `json:"confirmedAt"`
	QueryQueued        bool   `json:"queryQueued"`
}
