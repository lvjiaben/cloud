package v1

import "github.com/gogf/gf/v2/frame/g"

type FinancialManagementOverviewGetReq struct {
	g.Meta `path:"/admin/billing/overview" method:"get" tags:"IDC Financial Management" summary:"Get financial operations overview" dc:"Return bounded receivable and pending-review aggregates after exact overview permission." permission:"idc:admin:billing:overview"`
}
type FinancialManagementOverviewGetRes struct {
	Overview *FinancialManagementOverview `json:"overview" dc:"Bounded financial operations overview" eg:"{}"`
}

type FinancialManagementInvoiceListReq struct {
	Page         int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta       `path:"/admin/billing/invoices" method:"get" tags:"IDC Financial Management" summary:"List billing invoices" dc:"Return one encrypted-keyset page after exact invoice-list permission." permission:"idc:admin:billing:invoice:list"`
	CustomerNo   string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number" eg:"CUS_01J2ABCDEF"`
	StatusCode   string `json:"statusCode" dc:"Optional exact invoice status" eg:"pending_payment"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum invoices returned in this page" eg:"20"`
}
type FinancialManagementInvoiceListRes struct {
	Total      int                           `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Invoices   []*FinancialManagementInvoice `json:"invoices" dc:"Bounded authorized invoice summaries" eg:"[]"`
	NextCursor string                        `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type FinancialManagementInvoiceDetailReq struct {
	g.Meta     `path:"/admin/billing/invoices/{invoiceNo}" method:"get" tags:"IDC Financial Management" summary:"Get billing invoice detail" dc:"Return one invoice and one bounded immutable line page after exact invoice-detail permission." permission:"idc:admin:billing:invoice:query"`
	InvoiceNo  string `json:"invoiceNo" in:"path" v:"required|max-length:40" dc:"Stable invoice number" eg:"INV_01J2ABCDEF"`
	ItemCursor string `json:"itemCursor" v:"max-length:128" dc:"Optional encrypted invoice-line cursor" eg:"AQIDBA"`
	ItemLimit  int    `json:"itemLimit" d:"50" v:"min:1|max:100" dc:"Maximum immutable lines returned in this page" eg:"50"`
}
type FinancialManagementInvoiceDetailRes struct {
	Detail *FinancialManagementInvoiceDetail `json:"detail" dc:"Bounded authorized invoice aggregate" eg:"{}"`
}

type FinancialManagementTransactionListReq struct {
	Page                int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta              `path:"/admin/billing/transactions" method:"get" tags:"IDC Financial Management" summary:"List ledger transactions" dc:"Return one encrypted-keyset page after exact transaction-list permission." permission:"idc:admin:billing:transaction:list"`
	CustomerNo          string `json:"customerNo" v:"max-length:40" dc:"Optional derived related customer number" eg:"CUS_01J2ABCDEF"`
	TransactionTypeCode string `json:"transactionTypeCode" dc:"Optional exact transaction type" eg:"balance_payment"`
	CurrencyCode        string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor              string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit               int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum transactions returned in this page" eg:"20"`
}
type FinancialManagementTransactionListRes struct {
	Total        int                               `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Transactions []*FinancialManagementTransaction `json:"transactions" dc:"Bounded immutable transaction summaries" eg:"[]"`
	NextCursor   string                            `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type FinancialManagementTransactionDetailReq struct {
	g.Meta        `path:"/admin/billing/transactions/{transactionNo}" method:"get" tags:"IDC Financial Management" summary:"Get ledger transaction detail" dc:"Return one transaction and one bounded immutable entry page after exact transaction-detail permission." permission:"idc:admin:billing:transaction:query"`
	TransactionNo string `json:"transactionNo" in:"path" v:"required|max-length:40" dc:"Stable ledger transaction number" eg:"BTX_01J2ABCDEF"`
	EntryCursor   string `json:"entryCursor" v:"max-length:128" dc:"Optional encrypted ledger-entry cursor" eg:"AQIDBA"`
	EntryLimit    int    `json:"entryLimit" d:"50" v:"min:1|max:100" dc:"Maximum immutable entries returned in this page" eg:"50"`
}
type FinancialManagementTransactionDetailRes struct {
	Detail *FinancialManagementTransactionDetail `json:"detail" dc:"Bounded immutable transaction aggregate" eg:"{}"`
}

type FinancialManagementAdjustmentListReq struct {
	Page               int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta             `path:"/admin/billing/adjustments" method:"get" tags:"IDC Financial Management" summary:"List financial adjustments" dc:"Return one encrypted-keyset page after exact adjustment-list permission." permission:"idc:admin:billing:adjustment:list"`
	CustomerNo         string `json:"customerNo" v:"max-length:40" dc:"Optional exact target customer number" eg:"CUS_01J2ABCDEF"`
	AdjustmentTypeCode string `json:"adjustmentTypeCode" dc:"Optional exact adjustment type" eg:"manual_receipt"`
	StatusCode         string `json:"statusCode" dc:"Optional exact adjustment status" eg:"pending_review"`
	CurrencyCode       string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor             string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit              int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum adjustments returned in this page" eg:"20"`
}
type FinancialManagementAdjustmentListRes struct {
	Total       int                              `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Adjustments []*FinancialManagementAdjustment `json:"adjustments" dc:"Bounded authorized adjustment summaries" eg:"[]"`
	NextCursor  string                           `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type FinancialManagementAdjustmentDetailReq struct {
	g.Meta       `path:"/admin/billing/adjustments/{adjustmentNo}" method:"get" tags:"IDC Financial Management" summary:"Get financial adjustment detail" dc:"Return one adjustment and immutable review evidence after exact adjustment-detail permission." permission:"idc:admin:billing:adjustment:query"`
	AdjustmentNo string `json:"adjustmentNo" in:"path" v:"required|max-length:40" dc:"Stable adjustment request number" eg:"BAR_01J2ABCDEF"`
}

type FinancialManagementManualReceiptIntent struct {
	RequestNo                 string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Random action key stable across proof issuance and retries" eg:"receipt-01J2ABCDEF012345"`
	InvoiceNo                 string `json:"invoiceNo" v:"max-length:40" dc:"Optional exact invoice target; mutually exclusive with credit statement and balance targets" eg:"INV_01J2ABCDEF"`
	CreditStatementNo         string `json:"creditStatementNo" v:"max-length:40" dc:"Optional exact credit-statement target; mutually exclusive with invoice and balance targets" eg:"CST_01J2ABCDEF"`
	CustomerNo                string `json:"customerNo" v:"max-length:40" dc:"Balance-credit customer when invoice number is empty" eg:"CUS_01J2ABCDEF"`
	CurrencyCode              string `json:"currencyCode" v:"regex:^$|^[A-Z]{3}$" dc:"Balance-credit currency when invoice number is empty" eg:"CNY"`
	AmountMinor               string `json:"amountMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Exact positive receipt amount in decimal minor units" eg:"29900"`
	ReasonCode                string `json:"reasonCode" v:"max-length:64" dc:"Optional receipt reason; defaults to the operation code" eg:"offline_transfer"`
	ProofReference            string `json:"proofReference" v:"max-length:128" dc:"Optional private payment-proof reference; omitted when no proof is supplied" eg:"private-proof-01J2"`
	ReceivingAccountReference string `json:"receivingAccountReference" v:"max-length:128" dc:"Optional merchant receiving-account reference" eg:"merchant-collection-cny"`
	BusinessPurposeCode       string `json:"businessPurposeCode" v:"max-length:64" dc:"Optional receipt purpose; defaults to manual_receipt" eg:"invoice_payment"`
	EvidenceDigest            string `json:"evidenceDigest" v:"regex:^$|^[0-9a-f]{64}$" dc:"Optional annotation SHA-256 digest; absent input uses a system request trace" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type FinancialManagementInvoiceWaiverIntent struct {
	RequestNo      string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Random action key stable across proof issuance and retries" eg:"waiver-01J2ABCDEF012345"`
	InvoiceNo      string `json:"invoiceNo" v:"required|max-length:40" dc:"Exact invoice target" eg:"INV_01J2ABCDEF"`
	AmountMinor    string `json:"amountMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Exact positive waiver amount in decimal minor units" eg:"29900"`
	ReasonCode     string `json:"reasonCode" v:"max-length:64" dc:"Optional waiver reason; defaults to the operation code" eg:"service_compensation"`
	EvidenceDigest string `json:"evidenceDigest" v:"regex:^$|^[0-9a-f]{64}$" dc:"Optional annotation SHA-256 digest; absent input uses a system request trace" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type FinancialManagementAdjustmentReviewIntent struct {
	RequestNo      string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Random action key stable across proof issuance and retries" eg:"review-01J2ABCDEF012345"`
	DecisionCode   string `json:"decisionCode" v:"required|in:approved,rejected" dc:"Authorized decision for a historic pending request" eg:"approved"`
	CommentDigest  string `json:"commentDigest" v:"regex:^$|^[0-9a-f]{64}$" dc:"Optional annotation SHA-256 digest; absent input uses a system request trace" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	EvidenceDigest string `json:"evidenceDigest" v:"regex:^$|^[0-9a-f]{64}$" dc:"Optional annotation SHA-256 digest; absent input uses a system request trace" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type FinancialManagementAdjustmentPolicyIntent struct {
	RequestNo            string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Random action key stable across proof issuance and retries" eg:"policy-01J2ABCDEF012345"`
	CurrencyCode         string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured currency" eg:"CNY"`
	ReviewThresholdMinor string `json:"reviewThresholdMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Historic threshold metadata; no longer delays new operations" eg:"50000"`
}

type FinancialManagementActionProof struct {
	ProofToken string `json:"proofToken" dc:"Opaque session-, actor-, purpose-, and exact-preview-bound single-use proof" eg:"opaque-proof"`
	ExpiresAt  int64  `json:"expiresAt" dc:"Proof expiry as Unix timestamp in milliseconds" eg:"1720000000000"`
}

type FinancialManagementAdjustmentMutation struct {
	AdjustmentNo        string `json:"adjustmentNo" dc:"Stable adjustment request number" eg:"BAR_01J2ABCDEF"`
	AdjustmentTypeCode  string `json:"adjustmentTypeCode" dc:"Manual receipt or invoice waiver" eg:"manual_receipt"`
	StatusCode          string `json:"statusCode" dc:"Committed adjustment status" eg:"pending_review"`
	CustomerNo          string `json:"customerNo" dc:"Authoritative target customer" eg:"CUS_01J2ABCDEF"`
	InvoiceNo           string `json:"invoiceNo,omitempty" dc:"Authoritative target invoice, or empty" eg:"INV_01J2ABCDEF"`
	CreditStatementNo   string `json:"creditStatementNo,omitempty" dc:"Authoritative target credit statement, or empty" eg:"CST_01J2ABCDEF"`
	CurrencyCode        string `json:"currencyCode" dc:"Exact authoritative currency" eg:"CNY"`
	AmountMinor         string `json:"amountMinor" dc:"Exact adjustment amount in decimal minor units" eg:"29900"`
	ApprovalRequired    bool   `json:"approvalRequired" dc:"Whether this historic request was created with review required; new actions post directly" eg:"true"`
	LedgerTransactionNo string `json:"ledgerTransactionNo,omitempty" dc:"Posted immutable ledger transaction, or empty before approval" eg:"BTX_01J2ABCDEF"`
	Reused              bool   `json:"reused" dc:"Whether exact idempotent replay returned the committed result" eg:"false"`
}

type FinancialManagementAdjustmentPolicyMutation struct {
	PolicyNo             string `json:"policyNo" dc:"Stable immutable policy revision number" eg:"BAP_01J2ABCDEF"`
	CurrencyCode         string `json:"currencyCode" dc:"Exact policy currency" eg:"CNY"`
	ReviewThresholdMinor string `json:"reviewThresholdMinor" dc:"Exact review threshold in decimal minor units" eg:"50000"`
	Version              int64  `json:"version" dc:"Monotonic currency-local policy version" eg:"2"`
	Reused               bool   `json:"reused" dc:"Whether the current equal threshold was returned" eg:"false"`
}

type FinancialManagementManualReceiptProofReq struct {
	g.Meta `path:"/admin/billing/manual-receipts/proofs" method:"post" tags:"IDC Financial Management" summary:"Issue a manual-receipt action proof" dc:"Authorize the exact immutable receipt preview and issue a short-lived proof only after a current active session." permission:"idc:admin:billing:manual-receipt:create"`
	FinancialManagementManualReceiptIntent
}
type FinancialManagementManualReceiptProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use action proof" eg:"{}"`
}

type FinancialManagementManualReceiptCreateReq struct {
	g.Meta `path:"/admin/billing/manual-receipts" method:"post" tags:"IDC Financial Management" summary:"Create a manual receipt" dc:"Record verified offline money immediately after exact action authorization, with optional annotations and transactional audit." permission:"idc:admin:billing:manual-receipt:create"`
	FinancialManagementManualReceiptIntent
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Optional exact-preview proof; current action permission is sufficient" eg:"opaque-proof"`
}
type FinancialManagementManualReceiptCreateRes struct {
	Adjustment *FinancialManagementAdjustmentMutation `json:"adjustment" dc:"Committed receipt adjustment" eg:"{}"`
}

type FinancialManagementInvoiceWaiverProofReq struct {
	g.Meta `path:"/admin/billing/invoice-waivers/proofs" method:"post" tags:"IDC Financial Management" summary:"Issue an invoice-waiver action proof" dc:"Authorize the exact immutable waiver preview and issue a short-lived proof only after a current active session." permission:"idc:admin:billing:invoice-waiver:create"`
	FinancialManagementInvoiceWaiverIntent
}
type FinancialManagementInvoiceWaiverProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use action proof" eg:"{}"`
}

type FinancialManagementInvoiceWaiverCreateReq struct {
	g.Meta `path:"/admin/billing/invoice-waivers" method:"post" tags:"IDC Financial Management" summary:"Create a reviewed invoice waiver" dc:"Re-authorize and consume the exact-preview proof before recording a waiver request; payment facts cannot be forged." permission:"idc:admin:billing:invoice-waiver:create"`
	FinancialManagementInvoiceWaiverIntent
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Optional exact-preview proof; current action permission is sufficient" eg:"opaque-proof"`
}
type FinancialManagementInvoiceWaiverCreateRes struct {
	Adjustment *FinancialManagementAdjustmentMutation `json:"adjustment" dc:"Committed waiver adjustment" eg:"{}"`
}

type FinancialManagementAdjustmentReviewProofReq struct {
	g.Meta       `path:"/admin/billing/adjustments/{adjustmentNo}/review-proofs" method:"post" tags:"IDC Financial Management" summary:"Issue an adjustment-review action proof" dc:"Authorize the exact independent review preview and issue a short-lived proof after current session." permission:"idc:admin:billing:adjustment:review"`
	AdjustmentNo string `json:"adjustmentNo" in:"path" v:"required|max-length:40" dc:"Stable pending adjustment number" eg:"BAR_01J2ABCDEF"`
	FinancialManagementAdjustmentReviewIntent
}
type FinancialManagementAdjustmentReviewProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use action proof" eg:"{}"`
}

type FinancialManagementAdjustmentReviewReq struct {
	g.Meta       `path:"/admin/billing/adjustments/{adjustmentNo}/reviews" method:"post" tags:"IDC Financial Management" summary:"Review a pending financial adjustment" dc:"Authorize the current operator and process a historic request with atomic audit; the requester may process their own request." permission:"idc:admin:billing:adjustment:review"`
	AdjustmentNo string `json:"adjustmentNo" in:"path" v:"required|max-length:40" dc:"Stable pending adjustment number" eg:"BAR_01J2ABCDEF"`
	FinancialManagementAdjustmentReviewIntent
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Optional exact-preview proof; current action permission is sufficient" eg:"opaque-proof"`
}
type FinancialManagementAdjustmentReviewRes struct {
	Adjustment *FinancialManagementAdjustmentMutation `json:"adjustment" dc:"Committed processing result" eg:"{}"`
}

type FinancialManagementAdjustmentPolicyProofReq struct {
	g.Meta `path:"/admin/billing/adjustment-policies/proofs" method:"post" tags:"IDC Financial Management" summary:"Issue an adjustment-policy action proof" dc:"Authorize the exact currency-local review threshold preview and issue a short-lived proof after current session." permission:"idc:admin:billing:adjustment-policy:write"`
	FinancialManagementAdjustmentPolicyIntent
}
type FinancialManagementAdjustmentPolicyProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use action proof" eg:"{}"`
}

type FinancialManagementAdjustmentPolicyConfigureReq struct {
	g.Meta `path:"/admin/billing/adjustment-policies" method:"post" tags:"IDC Financial Management" summary:"Configure an adjustment review threshold" dc:"Re-authorize and consume the exact-preview proof before appending one immutable currency-local policy revision." permission:"idc:admin:billing:adjustment-policy:write"`
	FinancialManagementAdjustmentPolicyIntent
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Optional exact-preview proof; current action permission is sufficient" eg:"opaque-proof"`
}
type FinancialManagementAdjustmentPolicyConfigureRes struct {
	Policy *FinancialManagementAdjustmentPolicyMutation `json:"policy" dc:"Committed immutable policy revision" eg:"{}"`
}
type FinancialManagementAdjustmentDetailRes struct {
	Detail *FinancialManagementAdjustmentDetail `json:"detail" dc:"Authorized adjustment and review evidence" eg:"{}"`
}
