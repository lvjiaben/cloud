package v1

// FinancialBalance is one current customer account balance in an exact currency.
type FinancialBalance struct {
	AccountTypeCode       string `json:"accountTypeCode" dc:"Account type: customer_balance, customer_credit, customer_gift, or customer_commission" eg:"customer_balance"`
	CurrencyCode          string `json:"currencyCode" dc:"Exact configured ISO-style currency code" eg:"CNY"`
	StatusCode            string `json:"statusCode" dc:"Account status: active, frozen, or closed" eg:"active"`
	Withdrawable          bool   `json:"withdrawable" dc:"Whether this account type permits a separately governed withdrawal request" eg:"false"`
	BalanceMinor          string `json:"balanceMinor" dc:"Exact current balance in decimal minor units" eg:"125000"`
	HeldBalanceMinor      string `json:"heldBalanceMinor" dc:"Exact active held amount in decimal minor units" eg:"5000"`
	AvailableBalanceMinor string `json:"availableBalanceMinor" dc:"Exact spendable balance in decimal minor units" eg:"120000"`
	UpdatedAt             int64  `json:"updatedAt" dc:"Projection time as Unix timestamp in milliseconds" eg:"1720000000000"`
}

// FinancialReceivableSummary is one currency-local open receivable aggregate.
type FinancialReceivableSummary struct {
	CurrencyCode     string `json:"currencyCode" dc:"Exact receivable currency" eg:"CNY"`
	OutstandingMinor string `json:"outstandingMinor" dc:"Exact open receivable in decimal minor units" eg:"29900"`
}

// FinancialOverview is the bounded account financial landing projection.
type FinancialOverview struct {
	Balances                []*FinancialBalance           `json:"balances" dc:"At most 64 current customer-owned account projections" eg:"[]"`
	OpenInvoiceCount        int                           `json:"openInvoiceCount" dc:"Number of pending, partially paid, or overdue receivable invoices" eg:"2"`
	OpenReceivables         []*FinancialReceivableSummary `json:"openReceivables" dc:"Currency-local open receivable aggregates without conversion" eg:"[]"`
	AutomaticRenewalEnabled int                           `json:"automaticRenewalEnabled" dc:"Number of services with automatic renewal currently enabled" eg:"1"`
	RechargeAvailable       bool                          `json:"rechargeAvailable" dc:"Whether an installed payment owner currently exposes a verified recharge flow" eg:"false"`
	RechargeReasonCode      string                        `json:"rechargeReasonCode" dc:"Stable reason when recharge is unavailable, or empty" eg:"payment_capability_unavailable"`
}

// FinancialInvoice is one customer receivable invoice projection.
type FinancialInvoice struct {
	InvoiceNo              string `json:"invoiceNo" dc:"Stable customer invoice number" eg:"INV_01J2ABCDEF"`
	OrderNo                string `json:"orderNo,omitempty" dc:"Related order number, or empty" eg:"ORD_01J2ABCDEF"`
	SourceCode             string `json:"sourceCode" dc:"Invoice source: order, renewal, change, manual, consolidation, or migration" eg:"order"`
	CurrencyCode           string `json:"currencyCode" dc:"Exact invoice currency" eg:"CNY"`
	StatusCode             string `json:"statusCode" dc:"Invoice status: draft, pending_payment, partially_paid, paid, credit_settled, overdue, cancelled, partially_refunded, refunded, or bad_debt" eg:"pending_payment"`
	ItemCount              int    `json:"itemCount" dc:"Number of immutable invoice lines" eg:"1"`
	TotalAmountMinor       string `json:"totalAmountMinor" dc:"Exact total in decimal minor units" eg:"29900"`
	PaidAmountMinor        string `json:"paidAmountMinor" dc:"Exact customer-paid amount in decimal minor units" eg:"0"`
	WaivedAmountMinor      string `json:"waivedAmountMinor" dc:"Exact reviewed waiver amount in decimal minor units" eg:"0"`
	CreditAmountMinor      string `json:"creditAmountMinor" dc:"Exact approved-credit amount reserved for this invoice in decimal minor units; it is not cash paid" eg:"0"`
	OutstandingAmountMinor string `json:"outstandingAmountMinor" dc:"Exact amount still due in decimal minor units" eg:"29900"`
	RefundableAmountMinor  string `json:"refundableAmountMinor" dc:"Exact amount currently refundable in decimal minor units" eg:"0"`
	RefundedAmountMinor    string `json:"refundedAmountMinor" dc:"Exact amount already refunded in decimal minor units" eg:"0"`
	PaymentDueAt           int64  `json:"paymentDueAt" dc:"Payment deadline as Unix timestamp in milliseconds" eg:"1720000000000"`
	IssuedAt               int64  `json:"issuedAt,omitempty" dc:"Issue time as Unix timestamp in milliseconds, or zero" eg:"1719990000000"`
	SettledAt              int64  `json:"settledAt,omitempty" dc:"Settlement time as Unix timestamp in milliseconds, or zero" eg:"0"`
	CreatedAt              int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1719990000000"`
	CanPayFromBalance      bool   `json:"canPayFromBalance" dc:"Whether current same-currency available balance covers the exact outstanding amount" eg:"true"`
	CanMerge               bool   `json:"canMerge" dc:"Whether current header facts are eligible for authoritative merge revalidation" eg:"true"`
}

// FinancialInvoiceItem is one immutable invoice line.
type FinancialInvoiceItem struct {
	InvoiceItemNo    string `json:"invoiceItemNo" dc:"Stable invoice-item number" eg:"IVI_01J2ABCDEF"`
	LineNumber       int    `json:"lineNumber" dc:"Stable line sequence" eg:"1"`
	ItemTypeCode     string `json:"itemTypeCode" dc:"Line type: order_item, renewal, change, manual, or adjustment" eg:"order_item"`
	SourceReference  string `json:"sourceReference" dc:"Customer-visible source business reference" eg:"ORI_01J2ABCDEF"`
	DescriptionZhCN  string `json:"descriptionZhCN" dc:"Frozen Simplified Chinese line description" eg:"Cloud server zh-CN"`
	DescriptionEnUS  string `json:"descriptionEnUS" dc:"Frozen English line description" eg:"Cloud server"`
	SubtotalMinor    string `json:"subtotalMinor" dc:"Exact subtotal in decimal minor units" eg:"29900"`
	DiscountMinor    string `json:"discountMinor" dc:"Exact discount in decimal minor units" eg:"0"`
	NetAmountMinor   string `json:"netAmountMinor" dc:"Exact tax-exclusive amount in decimal minor units" eg:"29900"`
	TaxAmountMinor   string `json:"taxAmountMinor" dc:"Exact tax amount in decimal minor units" eg:"0"`
	TotalAmountMinor string `json:"totalAmountMinor" dc:"Exact line total in decimal minor units" eg:"29900"`
}

// FinancialInvoiceDetail combines an invoice with one bounded item page.
type FinancialInvoiceDetail struct {
	Invoice        *FinancialInvoice       `json:"invoice" dc:"Customer-owned receivable invoice header" eg:"{}"`
	Items          []*FinancialInvoiceItem `json:"items" dc:"At most 100 immutable invoice lines in this page" eg:"[]"`
	NextItemCursor string                  `json:"nextItemCursor" dc:"Encrypted next item-page cursor, or empty" eg:""`
}

// FinancialTransaction is one immutable transaction effect on a selected customer account type.
type FinancialTransaction struct {
	TransactionNo       string `json:"transactionNo" dc:"Stable immutable transaction number" eg:"BTX_01J2ABCDEF"`
	TransactionTypeCode string `json:"transactionTypeCode" dc:"Stable semantic transaction type" eg:"balance_payment"`
	CurrencyCode        string `json:"currencyCode" dc:"Exact transaction currency" eg:"CNY"`
	AccountTypeCode     string `json:"accountTypeCode" dc:"Selected customer account type" eg:"customer_balance"`
	AmountMinor         string `json:"amountMinor" dc:"Exact balanced transaction amount in decimal minor units" eg:"29900"`
	ChangeMinor         string `json:"changeMinor" dc:"Signed effect on the selected account in decimal minor units" eg:"-29900"`
	RelatedInvoiceNo    string `json:"relatedInvoiceNo,omitempty" dc:"Related receivable invoice number, or empty" eg:"INV_01J2ABCDEF"`
	PostedAt            int64  `json:"postedAt" dc:"Posting time as Unix timestamp in milliseconds" eg:"1720000000000"`
}

// FinancialRenewalService is one renewable long-term entitlement and its current automatic-renewal policy.
type FinancialRenewalService struct {
	EntitlementNo                string `json:"entitlementNo" dc:"Stable customer entitlement number" eg:"ENT_01J2ABCDEF"`
	ProductNameZhCN              string `json:"productNameZhCN" dc:"Simplified Chinese product name" eg:"Hong Kong Cloud Server zh-CN"`
	ProductNameEnUS              string `json:"productNameEnUS" dc:"English product name" eg:"Hong Kong Cloud Server"`
	CurrencyCode                 string `json:"currencyCode" dc:"Exact service billing currency" eg:"CNY"`
	EntitlementStatusCode        string `json:"entitlementStatusCode" dc:"Current customer entitlement status" eg:"active"`
	PeriodUnitCode               string `json:"periodUnitCode" dc:"Frozen billing period unit" eg:"month"`
	PeriodCount                  int    `json:"periodCount" dc:"Frozen billing period count" eg:"1"`
	EntitlementExpiresAt         int64  `json:"entitlementExpiresAt" dc:"Current entitlement expiry as Unix timestamp in milliseconds" eg:"1720000000000"`
	EntitlementVersion           int64  `json:"entitlementVersion" dc:"Optimistic entitlement version required by policy updates" eg:"3"`
	PolicyNo                     string `json:"policyNo,omitempty" dc:"Current immutable renewal-policy number, or empty" eg:"RPL_01J2ABCDEF"`
	PolicyVersion                int64  `json:"policyVersion,omitempty" dc:"Current immutable renewal-policy version, or zero" eg:"2"`
	AutoRenewEnabled             bool   `json:"autoRenewEnabled" dc:"Whether same-currency balance automatic renewal is enabled" eg:"true"`
	RenewalCycles                int    `json:"renewalCycles" dc:"Billing cycles requested per automatic renewal" eg:"1"`
	LeadSeconds                  int64  `json:"leadSeconds" dc:"Seconds before expiry when automatic renewal starts" eg:"604800"`
	MaximumAttempts              int    `json:"maximumAttempts" dc:"Maximum automatic renewal attempts" eg:"3"`
	RetryIntervalSeconds         int64  `json:"retryIntervalSeconds" dc:"Seconds between eligible retries" eg:"21600"`
	FailureNotificationEnabled   bool   `json:"failureNotificationEnabled" dc:"Whether terminal or balance-shortage outcomes request customer notification" eg:"true"`
	LatestAttemptOutcomeCode     string `json:"latestAttemptOutcomeCode,omitempty" dc:"Latest automatic-renewal outcome, or empty" eg:"insufficient_balance"`
	LatestAttemptFailureCode     string `json:"latestAttemptFailureCode,omitempty" dc:"Latest stable failure code, or empty" eg:"balance_insufficient"`
	LatestAttemptAt              int64  `json:"latestAttemptAt,omitempty" dc:"Latest attempt time as Unix timestamp in milliseconds, or zero" eg:"1719990000000"`
	NextAttemptAt                int64  `json:"nextAttemptAt,omitempty" dc:"Next eligible attempt time as Unix timestamp in milliseconds, or zero" eg:"1720010000000"`
	CanConfigureAutomaticRenewal bool   `json:"canConfigureAutomaticRenewal" dc:"Whether current authoritative entitlement facts permit a policy update" eg:"true"`
}

// FinancialBalancePayment is an exact customer-safe balance payment result.
type FinancialBalancePayment struct {
	TransactionNo          string `json:"transactionNo" dc:"Stable immutable ledger transaction number" eg:"BTX_01J2ABCDEF"`
	InvoiceNo              string `json:"invoiceNo" dc:"Settled current-customer invoice number" eg:"INV_01J2ABCDEF"`
	CurrencyCode           string `json:"currencyCode" dc:"Exact invoice and balance currency" eg:"CNY"`
	StatusCode             string `json:"statusCode" dc:"Current authoritative invoice status" eg:"paid"`
	PaidAmountMinor        string `json:"paidAmountMinor" dc:"Exact aggregate paid amount in decimal minor units" eg:"29900"`
	OutstandingAmountMinor string `json:"outstandingAmountMinor" dc:"Exact amount still due in decimal minor units" eg:"0"`
	BalanceMinor           string `json:"balanceMinor" dc:"Exact resulting same-currency balance in decimal minor units" eg:"70100"`
	AvailableBalanceMinor  string `json:"availableBalanceMinor" dc:"Exact resulting spendable balance in decimal minor units" eg:"70100"`
	Reused                 bool   `json:"reused" dc:"Whether this result safely replayed the same request" eg:"false"`
}

// FinancialCreditSettlement is an exact customer-safe approved-credit
// reservation and order-settlement result. It never presents credit as cash.
type FinancialCreditSettlement struct {
	SettlementNo      string `json:"settlementNo" dc:"Stable immutable financial settlement number" eg:"FST_01J2ABCDEF"`
	InvoiceNo         string `json:"invoiceNo" dc:"Settled current-customer invoice number" eg:"INV_01J2ABCDEF"`
	OrderNo           string `json:"orderNo" dc:"Order bound to the settled invoice" eg:"ORD_01J2ABCDEF"`
	CurrencyCode      string `json:"currencyCode" dc:"Exact invoice and approved-credit currency" eg:"CNY"`
	CreditAmountMinor string `json:"creditAmountMinor" dc:"Exact approved-credit amount reserved in decimal minor units; it is not cash paid" eg:"29900"`
	StatusCode        string `json:"statusCode" dc:"Current authoritative invoice status" eg:"credit_settled"`
	Reused            bool   `json:"reused" dc:"Whether the order's existing immutable settlement was safely replayed" eg:"false"`
}

// FinancialInvoiceMergeDestination is one newly derived receivable header.
type FinancialInvoiceMergeDestination struct {
	InvoiceNo              string `json:"invoiceNo" dc:"Stable derived invoice number" eg:"INV_01J2ABCDEF"`
	StatusCode             string `json:"statusCode" dc:"Derived invoice status" eg:"pending_payment"`
	TotalAmountMinor       string `json:"totalAmountMinor" dc:"Exact derived total in decimal minor units" eg:"49900"`
	OutstandingAmountMinor string `json:"outstandingAmountMinor" dc:"Exact derived outstanding amount in decimal minor units" eg:"49900"`
	PaymentDueAt           int64  `json:"paymentDueAt" dc:"Derived payment deadline as Unix timestamp in milliseconds" eg:"1720000000000"`
}

// FinancialInvoiceMerge is one immutable customer-safe merge projection.
type FinancialInvoiceMerge struct {
	TransformNo         string                              `json:"transformNo" dc:"Stable immutable invoice-transform number" eg:"BIT_01J2ABCDEF"`
	CurrencyCode        string                              `json:"currencyCode" dc:"Exact source and destination currency" eg:"CNY"`
	SourceInvoiceNos    []string                            `json:"sourceInvoiceNos" dc:"Canonical source invoice numbers" eg:"[\"INV_01J2ABCDEF\",\"INV_01J2ABCDEG\"]"`
	DestinationInvoices []*FinancialInvoiceMergeDestination `json:"destinationInvoices" dc:"Bounded derived invoice headers" eg:"[]"`
	Reused              bool                                `json:"reused" dc:"Whether this result safely replayed the same request" eg:"false"`
}

// FinancialRenewalPolicy is the current immutable automatic-renewal revision.
type FinancialRenewalPolicy struct {
	PolicyNo                   string `json:"policyNo" dc:"Stable immutable policy number" eg:"RPL_01J2ABCDEF"`
	EntitlementNo              string `json:"entitlementNo" dc:"Current-customer entitlement number" eg:"ENT_01J2ABCDEF"`
	PolicyVersion              int64  `json:"policyVersion" dc:"Current immutable policy revision" eg:"2"`
	EntitlementVersion         int64  `json:"entitlementVersion" dc:"New entitlement version required by the next edit" eg:"4"`
	AutoRenewEnabled           bool   `json:"autoRenewEnabled" dc:"Whether same-currency balance automatic renewal is enabled" eg:"true"`
	RenewalCycles              int    `json:"renewalCycles" dc:"Billing cycles requested per automatic renewal" eg:"1"`
	LeadSeconds                int64  `json:"leadSeconds" dc:"Seconds before expiry when automatic renewal starts" eg:"604800"`
	MaximumAttempts            int    `json:"maximumAttempts" dc:"Maximum automatic-renewal attempts" eg:"3"`
	RetryIntervalSeconds       int64  `json:"retryIntervalSeconds" dc:"Seconds between eligible retries" eg:"21600"`
	FailureNotificationEnabled bool   `json:"failureNotificationEnabled" dc:"Whether terminal or balance-shortage outcomes request customer notification" eg:"true"`
	Reused                     bool   `json:"reused" dc:"Whether the exact current policy was safely reused" eg:"false"`
}
