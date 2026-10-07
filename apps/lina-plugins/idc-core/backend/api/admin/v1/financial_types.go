package v1

type FinancialManagementReceivable struct {
	CurrencyCode     string `json:"currencyCode" dc:"Exact receivable currency" eg:"CNY"`
	OutstandingMinor string `json:"outstandingMinor" dc:"Exact open receivable in decimal minor units" eg:"125000"`
}

type FinancialManagementOverview struct {
	OpenInvoiceCount       int                              `json:"openInvoiceCount" dc:"Pending, partially paid, and overdue invoice count" eg:"12"`
	OverdueInvoiceCount    int                              `json:"overdueInvoiceCount" dc:"Overdue receivable invoice count" eg:"2"`
	PendingAdjustmentCount int                              `json:"pendingAdjustmentCount" dc:"Financial adjustments awaiting second-person review" eg:"1"`
	Receivables            []*FinancialManagementReceivable `json:"receivables" dc:"At most 64 currency-local receivable aggregates without conversion" eg:"[]"`
}

type FinancialManagementInvoice struct {
	InvoiceNo              string `json:"invoiceNo" dc:"Stable invoice number" eg:"INV_01J2ABCDEF"`
	CustomerNo             string `json:"customerNo" dc:"Owning customer number" eg:"CUS_01J2ABCDEF"`
	CustomerCode           string `json:"customerCode" dc:"Customer-facing short code" eg:"10008"`
	CustomerDisplayName    string `json:"customerDisplayName" dc:"Safe customer display name" eg:"Example customer"`
	CustomerPrimaryPhone   string `json:"customerPrimaryPhone" dc:"Safe primary phone summary" eg:"138****8000"`
	OrderNo                string `json:"orderNo,omitempty" dc:"Related order number, or empty" eg:"ORD_01J2ABCDEF"`
	SourceCode             string `json:"sourceCode" dc:"Stable invoice source code" eg:"order"`
	CurrencyCode           string `json:"currencyCode" dc:"Exact invoice currency" eg:"CNY"`
	StatusCode             string `json:"statusCode" dc:"Current authoritative invoice status" eg:"pending_payment"`
	ItemCount              int    `json:"itemCount" dc:"Immutable invoice-line count" eg:"2"`
	TotalAmountMinor       string `json:"totalAmountMinor" dc:"Exact invoice total in decimal minor units" eg:"29900"`
	PaidAmountMinor        string `json:"paidAmountMinor" dc:"Exact paid amount in decimal minor units" eg:"0"`
	WaivedAmountMinor      string `json:"waivedAmountMinor" dc:"Exact reviewed waiver amount in decimal minor units" eg:"0"`
	CreditAmountMinor      string `json:"creditAmountMinor" dc:"Exact approved-credit amount reserved for this invoice in decimal minor units; it is not cash paid" eg:"0"`
	OutstandingAmountMinor string `json:"outstandingAmountMinor" dc:"Exact amount still due in decimal minor units" eg:"29900"`
	RefundableAmountMinor  string `json:"refundableAmountMinor" dc:"Exact currently refundable amount in decimal minor units" eg:"0"`
	RefundedAmountMinor    string `json:"refundedAmountMinor" dc:"Exact already refunded amount in decimal minor units" eg:"0"`
	PaymentDueAt           int64  `json:"paymentDueAt" dc:"Payment deadline as Unix timestamp in milliseconds" eg:"1720000000000"`
	IssuedAt               int64  `json:"issuedAt,omitempty" dc:"Issue time as Unix timestamp in milliseconds, or zero" eg:"1719990000000"`
	SettledAt              int64  `json:"settledAt,omitempty" dc:"Settlement time as Unix timestamp in milliseconds, or zero" eg:"0"`
	CreatedAt              int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1719990000000"`
}

type FinancialManagementInvoiceItem struct {
	InvoiceItemNo    string `json:"invoiceItemNo" dc:"Stable invoice-item number" eg:"IVI_01J2ABCDEF"`
	LineNumber       int    `json:"lineNumber" dc:"Stable line sequence" eg:"1"`
	ItemTypeCode     string `json:"itemTypeCode" dc:"Stable invoice-line type" eg:"order_item"`
	SourceReference  string `json:"sourceReference" dc:"Source business reference" eg:"ORI_01J2ABCDEF"`
	DescriptionZhCN  string `json:"descriptionZhCN" dc:"Frozen Simplified Chinese description" eg:"Cloud server zh-CN"`
	DescriptionEnUS  string `json:"descriptionEnUS" dc:"Frozen English description" eg:"Cloud server"`
	SubtotalMinor    string `json:"subtotalMinor" dc:"Exact subtotal in decimal minor units" eg:"29900"`
	DiscountMinor    string `json:"discountMinor" dc:"Exact discount in decimal minor units" eg:"0"`
	NetAmountMinor   string `json:"netAmountMinor" dc:"Exact tax-exclusive amount in decimal minor units" eg:"29900"`
	TaxAmountMinor   string `json:"taxAmountMinor" dc:"Exact tax in decimal minor units" eg:"0"`
	TotalAmountMinor string `json:"totalAmountMinor" dc:"Exact line total in decimal minor units" eg:"29900"`
}

type FinancialManagementInvoiceDetail struct {
	Invoice        *FinancialManagementInvoice       `json:"invoice" dc:"Authorized invoice header" eg:"{}"`
	Items          []*FinancialManagementInvoiceItem `json:"items" dc:"At most 100 immutable invoice lines in this page" eg:"[]"`
	NextItemCursor string                            `json:"nextItemCursor" dc:"Encrypted next item-page cursor, or empty" eg:""`
}

type FinancialManagementTransaction struct {
	TransactionNo        string `json:"transactionNo" dc:"Stable immutable transaction number" eg:"BTX_01J2ABCDEF"`
	TransactionTypeCode  string `json:"transactionTypeCode" dc:"Stable semantic transaction type" eg:"balance_payment"`
	CustomerNo           string `json:"customerNo,omitempty" dc:"Derived related customer number, or empty for merchant-only entries" eg:"CUS_01J2ABCDEF"`
	CustomerCode         string `json:"customerCode,omitempty" dc:"Customer-facing short code" eg:"10008"`
	CustomerDisplayName  string `json:"customerDisplayName,omitempty" dc:"Safe customer display name" eg:"Example customer"`
	CustomerPrimaryPhone string `json:"customerPrimaryPhone,omitempty" dc:"Safe primary phone summary" eg:"138****8000"`
	CurrencyCode         string `json:"currencyCode" dc:"Exact transaction currency" eg:"CNY"`
	AmountMinor          string `json:"amountMinor" dc:"Exact balanced transaction amount in decimal minor units" eg:"29900"`
	RelatedInvoiceNo     string `json:"relatedInvoiceNo,omitempty" dc:"Derived related invoice number, or empty" eg:"INV_01J2ABCDEF"`
	EntryCount           int    `json:"entryCount" dc:"Immutable ledger-entry count" eg:"2"`
	PostedAt             int64  `json:"postedAt" dc:"Posting time as Unix timestamp in milliseconds" eg:"1720000000000"`
}

type FinancialManagementTransactionEntry struct {
	EntryNo         string `json:"entryNo" dc:"Stable immutable ledger-entry number" eg:"BLE_01J2ABCDEF"`
	LineNumber      int    `json:"lineNumber" dc:"Stable transaction line sequence" eg:"1"`
	AccountNo       string `json:"accountNo" dc:"Stable ledger account number" eg:"BAC_01J2ABCDEF"`
	OwnerTypeCode   string `json:"ownerTypeCode" dc:"Ledger owner type" eg:"customer"`
	OwnerReference  string `json:"ownerReference" dc:"Ledger owner business reference" eg:"CUS_01J2ABCDEF"`
	CustomerNo      string `json:"customerNo,omitempty" dc:"Customer number for customer-owned accounts, or empty" eg:"CUS_01J2ABCDEF"`
	AccountTypeCode string `json:"accountTypeCode" dc:"Stable account type" eg:"customer_balance"`
	DirectionCode   string `json:"directionCode" dc:"Entry direction: debit or credit" eg:"debit"`
	EntryRoleCode   string `json:"entryRoleCode" dc:"Stable semantic entry role" eg:"balance"`
	AmountMinor     string `json:"amountMinor" dc:"Exact entry amount in decimal minor units" eg:"29900"`
	InvoiceNo       string `json:"invoiceNo,omitempty" dc:"Related invoice number, or empty" eg:"INV_01J2ABCDEF"`
	InvoiceItemNo   string `json:"invoiceItemNo,omitempty" dc:"Related invoice-item number, or empty" eg:"IVI_01J2ABCDEF"`
	ReasonCode      string `json:"reasonCode" dc:"Stable posting reason code" eg:"invoice_balance_payment"`
}

type FinancialManagementTransactionDetail struct {
	TransactionNo         string                                 `json:"transactionNo" dc:"Stable immutable transaction number" eg:"BTX_01J2ABCDEF"`
	TransactionTypeCode   string                                 `json:"transactionTypeCode" dc:"Stable semantic transaction type" eg:"balance_payment"`
	CurrencyCode          string                                 `json:"currencyCode" dc:"Exact transaction currency" eg:"CNY"`
	SourceTypeCode        string                                 `json:"sourceTypeCode" dc:"Trusted source-owner type" eg:"invoice_balance_payment"`
	SourceReference       string                                 `json:"sourceReference" dc:"Trusted source business reference" eg:"payment-confirmation-01J2"`
	OriginalTransactionNo string                                 `json:"originalTransactionNo,omitempty" dc:"Original transaction for a reversal, or empty" eg:"BTX_01J2ABCDE0"`
	EntryCount            int                                    `json:"entryCount" dc:"Immutable ledger-entry count" eg:"2"`
	AmountMinor           string                                 `json:"amountMinor" dc:"Exact balanced transaction amount in decimal minor units" eg:"29900"`
	PostedBy              int                                    `json:"postedBy" dc:"Posting actor identifier, or zero for trusted system posting" eg:"1001"`
	PostedAt              int64                                  `json:"postedAt" dc:"Posting time as Unix timestamp in milliseconds" eg:"1720000000000"`
	Entries               []*FinancialManagementTransactionEntry `json:"entries" dc:"At most 100 immutable ledger entries in this page" eg:"[]"`
	NextEntryCursor       string                                 `json:"nextEntryCursor" dc:"Encrypted next entry-page cursor, or empty" eg:""`
}

type FinancialManagementAdjustment struct {
	AdjustmentNo         string `json:"adjustmentNo" dc:"Stable adjustment request number" eg:"BAR_01J2ABCDEF"`
	AdjustmentTypeCode   string `json:"adjustmentTypeCode" dc:"Adjustment type: manual_receipt or invoice_waiver" eg:"manual_receipt"`
	StatusCode           string `json:"statusCode" dc:"Adjustment lifecycle status" eg:"pending_review"`
	CustomerNo           string `json:"customerNo" dc:"Target customer number" eg:"CUS_01J2ABCDEF"`
	CustomerCode         string `json:"customerCode" dc:"Customer-facing short code" eg:"10008"`
	CustomerDisplayName  string `json:"customerDisplayName" dc:"Safe customer display name" eg:"Example customer"`
	CustomerPrimaryPhone string `json:"customerPrimaryPhone" dc:"Safe primary phone summary" eg:"138****8000"`
	InvoiceNo            string `json:"invoiceNo,omitempty" dc:"Target invoice number, or empty for a balance receipt" eg:"INV_01J2ABCDEF"`
	CurrencyCode         string `json:"currencyCode" dc:"Exact adjustment currency" eg:"CNY"`
	AmountMinor          string `json:"amountMinor" dc:"Exact adjustment amount in decimal minor units" eg:"29900"`
	ReasonCode           string `json:"reasonCode" dc:"Stable reviewed reason code" eg:"offline_transfer"`
	ApprovalRequired     bool   `json:"approvalRequired" dc:"Whether a distinct second-person decision is required" eg:"true"`
	RequestedBy          int    `json:"requestedBy" dc:"Direct staff requester identifier" eg:"1001"`
	LedgerTransactionNo  string `json:"ledgerTransactionNo,omitempty" dc:"Posted ledger transaction, or empty before posting" eg:"BTX_01J2ABCDEF"`
	CreatedAt            int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1719990000000"`
	UpdatedAt            int64  `json:"updatedAt" dc:"Latest lifecycle time as Unix timestamp in milliseconds" eg:"1720000000000"`
}

type FinancialManagementAdjustmentDetail struct {
	Adjustment                *FinancialManagementAdjustment `json:"adjustment" dc:"Authorized adjustment header" eg:"{}"`
	EvidenceDigest            string                         `json:"evidenceDigest" dc:"Lowercase SHA-256 digest of frozen request evidence" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ProofReference            string                         `json:"proofReference,omitempty" dc:"Private payment-proof object reference, or empty" eg:"private-proof-01J2"`
	ReceivingAccountReference string                         `json:"receivingAccountReference,omitempty" dc:"Merchant receiving-account reference, or empty" eg:"merchant-collection-cny"`
	BusinessPurposeCode       string                         `json:"businessPurposeCode,omitempty" dc:"Stable verified receipt purpose, or empty" eg:"invoice_payment"`
	PolicyNo                  string                         `json:"policyNo,omitempty" dc:"Frozen approval-policy number, or empty when fail-closed" eg:"BAP_01J2ABCDEF"`
	PolicyVersion             int64                          `json:"policyVersion,omitempty" dc:"Frozen approval-policy version, or zero" eg:"2"`
	Version                   int64                          `json:"version" dc:"Optimistic adjustment lifecycle version" eg:"1"`
	ReviewerID                int                            `json:"reviewerId,omitempty" dc:"Distinct review actor, or zero before review" eg:"1002"`
	DecisionCode              string                         `json:"decisionCode,omitempty" dc:"Review decision, or empty before review" eg:"approved"`
	CommentDigest             string                         `json:"commentDigest,omitempty" dc:"Digest of frozen review comment evidence, or empty" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ReviewEvidenceDigest      string                         `json:"reviewEvidenceDigest,omitempty" dc:"Digest of frozen review evidence, or empty" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	DecidedAt                 int64                          `json:"decidedAt,omitempty" dc:"Decision time as Unix timestamp in milliseconds, or zero" eg:"1720000000000"`
	PostedAt                  int64                          `json:"postedAt,omitempty" dc:"Posting time as Unix timestamp in milliseconds, or zero" eg:"1720000000000"`
}
