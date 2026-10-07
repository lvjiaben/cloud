package v1

import "github.com/gogf/gf/v2/frame/g"

type FinancialOverviewGetReq struct {
	g.Meta `path:"/account/billing/overview" method:"get" tags:"IDC Customer Billing" summary:"Get my financial overview" dc:"Return bounded current-customer balances, open receivables, automatic-renewal count, and verified recharge availability in one request."`
}
type FinancialOverviewGetRes struct {
	Overview *FinancialOverview `json:"overview" dc:"Current customer financial overview" eg:"{}"`
}

type FinancialInvoiceListReq struct {
	g.Meta       `path:"/account/billing/invoices" method:"get" tags:"IDC Customer Billing" summary:"List my receivable invoices" dc:"Return one encrypted-keyset page owned by the current active customer without an expensive total count."`
	StatusCode   string `json:"statusCode" dc:"Optional exact invoice status; empty includes all statuses" eg:"pending_payment"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency; empty includes all currencies without conversion" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:50" dc:"Maximum invoices returned in this page" eg:"20"`
}
type FinancialInvoiceListRes struct {
	Invoices   []*FinancialInvoice `json:"invoices" dc:"Bounded current-customer invoice summaries" eg:"[]"`
	NextCursor string              `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type FinancialInvoiceDetailReq struct {
	g.Meta     `path:"/account/billing/invoices/{invoiceNo}" method:"get" tags:"IDC Customer Billing" summary:"Get my receivable invoice" dc:"Return one customer-owned invoice header and one bounded immutable line page without frontend waterfall requests."`
	InvoiceNo  string `json:"invoiceNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer invoice number" eg:"INV_01J2ABCDEF"`
	ItemCursor string `json:"itemCursor" v:"max-length:128" dc:"Optional encrypted invoice-line cursor" eg:"AQIDBA"`
	ItemLimit  int    `json:"itemLimit" d:"50" v:"min:1|max:100" dc:"Maximum immutable lines returned in this page" eg:"50"`
}
type FinancialInvoiceDetailRes struct {
	Detail *FinancialInvoiceDetail `json:"detail" dc:"Bounded customer-safe invoice aggregate" eg:"{}"`
}

type FinancialTransactionListReq struct {
	g.Meta       `path:"/account/billing/transactions" method:"get" tags:"IDC Customer Billing" summary:"List my financial transactions" dc:"Return one encrypted-keyset page of immutable effects on a selected current-customer account type."`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact currency; empty includes all currencies without conversion" eg:"CNY"`
	AccountType  string `json:"accountType" d:"customer_balance" dc:"Customer account type: customer_balance, customer_credit, customer_gift, or customer_commission" eg:"customer_balance"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:50" dc:"Maximum transactions returned in this page" eg:"20"`
}
type FinancialTransactionListRes struct {
	Transactions []*FinancialTransaction `json:"transactions" dc:"Bounded immutable customer-account transaction effects" eg:"[]"`
	NextCursor   string                  `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type FinancialRenewalServiceListReq struct {
	g.Meta     `path:"/account/billing/renewal-entitlements" method:"get" tags:"IDC Customer Billing" summary:"List my renewable entitlements" dc:"Return one encrypted-keyset page of current-customer renewable long-term entitlements, current policies, and latest automatic attempt outcomes."`
	StatusCode string `json:"statusCode" dc:"Optional exact entitlement status; empty includes all renewable entitlements" eg:"active"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit      int    `json:"limit" d:"20" v:"min:1|max:50" dc:"Maximum services returned in this page" eg:"20"`
}
type FinancialRenewalServiceListRes struct {
	Entitlements []*FinancialRenewalService `json:"entitlements" dc:"Bounded current-customer renewable long-term entitlements" eg:"[]"`
	NextCursor   string                     `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type FinancialInvoiceBalancePaymentCreateReq struct {
	g.Meta         `path:"/account/billing/invoices/{invoiceNo}/balance-payments" method:"post" tags:"IDC Customer Billing" summary:"Pay my invoice from balance" dc:"Atomically settle the exact current outstanding amount of one owned invoice from its same-currency available balance."`
	InvoiceNo      string `json:"invoiceNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer invoice number" eg:"INV_01J2ABCDEF"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random request key stable across retries; the raw value is never persisted" eg:"balance-pay-01J2ABCDEF"`
}
type FinancialInvoiceBalancePaymentCreateRes struct {
	Payment *FinancialBalancePayment `json:"payment" dc:"Atomic customer-safe balance payment result" eg:"{}"`
}

type FinancialInvoiceCreditSettlementCreateReq struct {
	g.Meta    `path:"/account/billing/invoices/{invoiceNo}/credit-settlements" method:"post" tags:"IDC Customer Billing" summary:"Settle my order invoice from approved credit" dc:"Atomically reserve the exact current invoice total from the current customer's active same-currency approved credit account and settle its bound order. Amount, credit account, and order are derived by the server."`
	InvoiceNo string `json:"invoiceNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer order invoice number" eg:"INV_01J2ABCDEF"`
}
type FinancialInvoiceCreditSettlementCreateRes struct {
	Settlement *FinancialCreditSettlement `json:"settlement" dc:"Atomic customer-safe approved-credit settlement result" eg:"{}"`
}

type FinancialInvoiceMergeCreateReq struct {
	g.Meta         `path:"/account/billing/invoice-merges" method:"post" tags:"IDC Customer Billing" summary:"Merge my invoices" dc:"Atomically replace two or more untouched, payable, same-currency current-customer invoices with one derived invoice."`
	InvoiceNos     []string `json:"invoiceNos" v:"required" dc:"Two to one hundred unique current-customer invoice numbers" eg:"[\"INV_01J2ABCDEF\",\"INV_01J2ABCDEG\"]"`
	IdempotencyKey string   `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random request key stable across retries; the raw value is never persisted" eg:"invoice-merge-01J2ABCDEF"`
}
type FinancialInvoiceMergeCreateRes struct {
	Merge *FinancialInvoiceMerge `json:"merge" dc:"Immutable customer-safe invoice merge result" eg:"{}"`
}

type FinancialRenewalPolicyUpdateReq struct {
	g.Meta                     `path:"/account/billing/renewal-entitlements/{entitlementNo}/policy" method:"put" tags:"IDC Customer Billing" summary:"Configure my automatic renewal" dc:"Append one customer-owned automatic-renewal policy revision using optimistic entitlement concurrency."`
	EntitlementNo              string `json:"entitlementNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer entitlement number" eg:"ENT_01J2ABCDEF"`
	ExpectedEntitlementVersion int64  `json:"expectedEntitlementVersion" v:"required|min:1" dc:"Entitlement version observed by the customer before editing" eg:"3"`
	AutoRenewEnabled           bool   `json:"autoRenewEnabled" dc:"Whether same-currency balance automatic renewal is enabled" eg:"true"`
	RenewalCycles              int    `json:"renewalCycles" v:"required|min:1|max:120" dc:"Billing cycles requested per automatic renewal" eg:"1"`
	LeadSeconds                int64  `json:"leadSeconds" v:"required|min:3600|max:31536000" dc:"Seconds before expiry when automatic renewal starts" eg:"604800"`
	MaximumAttempts            int    `json:"maximumAttempts" v:"required|min:1|max:32" dc:"Maximum automatic-renewal attempts" eg:"3"`
	RetryIntervalSeconds       int64  `json:"retryIntervalSeconds" v:"required|min:300|max:2592000" dc:"Seconds between eligible retries" eg:"21600"`
	FailureNotificationEnabled bool   `json:"failureNotificationEnabled" dc:"Whether terminal or balance-shortage outcomes request customer notification" eg:"true"`
}
type FinancialRenewalPolicyUpdateRes struct {
	Policy *FinancialRenewalPolicy `json:"policy" dc:"Current immutable automatic-renewal policy revision" eg:"{}"`
}
