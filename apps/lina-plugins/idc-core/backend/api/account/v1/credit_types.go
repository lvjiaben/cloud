// This file defines customer-safe credit response types shared by account endpoints.

package v1

// CreditAccountTerms exposes the immutable terms currently bound to a customer credit account.
type CreditAccountTerms struct {
	TermsNo                 string `json:"termsNo" dc:"Stable immutable credit-terms number" eg:"CTR_01J2ABCDEF"`
	TermsVersion            int64  `json:"termsVersion" dc:"Monotonic terms version" eg:"1"`
	TotalLimitMinor         string `json:"totalLimitMinor" dc:"Exact approved limit in decimal minor units" eg:"100000"`
	BillingDay              int    `json:"billingDay" dc:"Merchant-timezone monthly billing day from 1 through 28" eg:"10"`
	RepaymentPeriodDays     int    `json:"repaymentPeriodDays" dc:"Frozen repayment period in days" eg:"15"`
	GracePeriodDays         int    `json:"gracePeriodDays" dc:"Frozen delinquency grace period in days" eg:"3"`
	DailyLateFeeBasisPoints int    `json:"dailyLateFeeBasisPoints" dc:"Daily late-fee rate in basis points" eg:"5"`
	MaximumLateFeeMinor     string `json:"maximumLateFeeMinor" dc:"Frozen total late-fee cap in decimal minor units" eg:"5000"`
	EffectiveFrom           int64  `json:"effectiveFrom" dc:"Terms start as Unix timestamp in milliseconds" eg:"1720000000000"`
	EffectiveUntil          int64  `json:"effectiveUntil" dc:"Terms end as Unix timestamp in milliseconds" eg:"1751536000000"`
}

// CreditDelinquency is the latest bounded delinquency projection for a customer credit account.
type CreditDelinquency struct {
	CaseNo                 string `json:"caseNo" dc:"Stable delinquency case number" eg:"CDC_01J2ABCDEF"`
	StatementNo            string `json:"statementNo" dc:"Related credit statement number" eg:"CST_01J2ABCDEF"`
	StatusCode             string `json:"statusCode" dc:"Current delinquency lifecycle code: grace_period, frozen, service_action_pending, written_off, or resolved" eg:"grace_period"`
	PrincipalMinor         string `json:"principalMinor" dc:"Exact delinquent principal in decimal minor units" eg:"20000"`
	LateFeeMinor           string `json:"lateFeeMinor" dc:"Exact accrued late fee in decimal minor units" eg:"100"`
	OutstandingAmountMinor string `json:"outstandingAmountMinor" dc:"Exact case outstanding amount in decimal minor units" eg:"20100"`
	OpenedAt               int64  `json:"openedAt" dc:"Case opening time as Unix timestamp in milliseconds" eg:"1720000000000"`
	GraceUntil             int64  `json:"graceUntil" dc:"Frozen grace deadline as Unix timestamp in milliseconds" eg:"1720259200000"`
	ResolvedAt             int64  `json:"resolvedAt,omitempty" dc:"Resolution time as Unix timestamp in milliseconds, or zero" eg:"0"`
}

// CreditAccount is one current-customer, currency-local credit account projection.
type CreditAccount struct {
	CreditAccountNo          string              `json:"creditAccountNo" dc:"Stable current-customer credit account number" eg:"CRA_01J2ABCDEF"`
	CurrencyCode             string              `json:"currencyCode" dc:"Exact credit currency" eg:"CNY"`
	StatusCode               string              `json:"statusCode" dc:"Current credit account lifecycle code: pending_activation, active, frozen, expired, closed, or bad_debt" eg:"active"`
	TotalLimitMinor          string              `json:"totalLimitMinor" dc:"Exact total credit limit in decimal minor units" eg:"100000"`
	AvailableLimitMinor      string              `json:"availableLimitMinor" dc:"Exact currently available limit in decimal minor units" eg:"70000"`
	UnbilledPrincipalMinor   string              `json:"unbilledPrincipalMinor" dc:"Exact unbilled principal in decimal minor units" eg:"10000"`
	BilledPrincipalMinor     string              `json:"billedPrincipalMinor" dc:"Exact billed principal in decimal minor units" eg:"20000"`
	DelinquentPrincipalMinor string              `json:"delinquentPrincipalMinor" dc:"Billed principal currently delinquent in decimal minor units" eg:"0"`
	WrittenOffPrincipalMinor string              `json:"writtenOffPrincipalMinor" dc:"Exact written-off principal in decimal minor units" eg:"0"`
	ValidFrom                int64               `json:"validFrom" dc:"Account validity start as Unix timestamp in milliseconds" eg:"1720000000000"`
	ValidUntil               int64               `json:"validUntil" dc:"Account validity end as Unix timestamp in milliseconds" eg:"1751536000000"`
	FreezeReasonCode         string              `json:"freezeReasonCode,omitempty" dc:"Stable freeze reason, or empty" eg:"credit_delinquency"`
	FrozenAt                 int64               `json:"frozenAt,omitempty" dc:"Freeze time as Unix timestamp in milliseconds, or zero" eg:"0"`
	Version                  int64               `json:"version" dc:"Current optimistic account version" eg:"3"`
	UpdatedAt                int64               `json:"updatedAt" dc:"Projection time as Unix timestamp in milliseconds" eg:"1720000000000"`
	Terms                    *CreditAccountTerms `json:"terms" dc:"Current immutable terms" eg:"{}"`
	LatestDelinquency        *CreditDelinquency  `json:"latestDelinquency,omitempty" dc:"Latest delinquency case, or absent" eg:"{}"`
}

// CreditOverview summarizes the current customer's bounded credit position.
type CreditOverview struct {
	Accounts                []*CreditAccount `json:"accounts" dc:"At most 64 current-customer currency-local credit accounts" eg:"[]"`
	PendingApplicationCount int              `json:"pendingApplicationCount" dc:"Current-customer credit applications awaiting review" eg:"1"`
	OpenStatementCount      int              `json:"openStatementCount" dc:"Open, partially paid, or overdue credit statements" eg:"2"`
	OverdueStatementCount   int              `json:"overdueStatementCount" dc:"Overdue credit statements" eg:"1"`
}

// CreditApplication is one current-customer credit application projection.
type CreditApplication struct {
	ApplicationNo         string `json:"applicationNo" dc:"Stable current-customer credit application number" eg:"CAP_01J2ABCDEF"`
	CurrencyCode          string `json:"currencyCode" dc:"Exact requested currency" eg:"CNY"`
	PolicyNo              string `json:"policyNo" dc:"Frozen policy revision number" eg:"CRP_01J2ABCDEF"`
	PolicyVersion         int64  `json:"policyVersion" dc:"Frozen policy version" eg:"2"`
	RequestedLimitMinor   string `json:"requestedLimitMinor" dc:"Exact requested limit in decimal minor units" eg:"100000"`
	RequestedValidFrom    int64  `json:"requestedValidFrom" dc:"Requested validity start as Unix timestamp in milliseconds" eg:"1720000000000"`
	RequestedValidUntil   int64  `json:"requestedValidUntil" dc:"Requested validity end as Unix timestamp in milliseconds" eg:"1751536000000"`
	RequiredApprovalCount int    `json:"requiredApprovalCount" dc:"Frozen required reviewer count" eg:"2"`
	ReviewCount           int    `json:"reviewCount" dc:"Recorded independent reviewer count" eg:"1"`
	StatusCode            string `json:"statusCode" dc:"Current application lifecycle code: pending_review, approved, rejected, cancelled, or expired" eg:"pending_review"`
	CreditAccountNo       string `json:"creditAccountNo,omitempty" dc:"Activated credit account number after approval, or empty" eg:"CRA_01J2ABCDEF"`
	CreatedAt             int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1720000000000"`
	UpdatedAt             int64  `json:"updatedAt" dc:"Last lifecycle update as Unix timestamp in milliseconds" eg:"1720000000000"`
	Reused                bool   `json:"reused,omitempty" dc:"Whether an exact application retry reused the committed result" eg:"false"`
}

// CreditStatement is one current-customer statement header with safe repayment eligibility.
type CreditStatement struct {
	StatementNo            string `json:"statementNo" dc:"Stable current-customer credit statement number" eg:"CST_01J2ABCDEF"`
	CreditAccountNo        string `json:"creditAccountNo" dc:"Owning credit account number" eg:"CRA_01J2ABCDEF"`
	CurrencyCode           string `json:"currencyCode" dc:"Exact statement currency" eg:"CNY"`
	PeriodStartedAt        int64  `json:"periodStartedAt" dc:"Statement period start as Unix timestamp in milliseconds" eg:"1717200000000"`
	PeriodEndedAt          int64  `json:"periodEndedAt" dc:"Statement period end as Unix timestamp in milliseconds" eg:"1719792000000"`
	LineCount              int    `json:"lineCount" dc:"Immutable statement line count" eg:"2"`
	PrincipalMinor         string `json:"principalMinor" dc:"Exact statement principal in decimal minor units" eg:"20000"`
	FeeMinor               string `json:"feeMinor" dc:"Exact statement late fees in decimal minor units" eg:"100"`
	AdjustmentMinor        string `json:"adjustmentMinor" dc:"Signed statement adjustment in decimal minor units" eg:"0"`
	TotalAmountMinor       string `json:"totalAmountMinor" dc:"Exact statement total in decimal minor units" eg:"20100"`
	PaidAmountMinor        string `json:"paidAmountMinor" dc:"Exact repaid amount in decimal minor units" eg:"10000"`
	WrittenOffAmountMinor  string `json:"writtenOffAmountMinor" dc:"Exact written-off amount in decimal minor units" eg:"0"`
	OutstandingAmountMinor string `json:"outstandingAmountMinor" dc:"Exact outstanding amount in decimal minor units" eg:"10100"`
	StatusCode             string `json:"statusCode" dc:"Current statement lifecycle code: open, partially_paid, paid, overdue, written_off, cancelled, or refunded" eg:"partially_paid"`
	IssuedAt               int64  `json:"issuedAt" dc:"Issue time as Unix timestamp in milliseconds" eg:"1719792000000"`
	PaymentDueAt           int64  `json:"paymentDueAt" dc:"Payment deadline as Unix timestamp in milliseconds" eg:"1721088000000"`
	OverdueAt              int64  `json:"overdueAt,omitempty" dc:"Overdue transition time as Unix timestamp in milliseconds, or zero" eg:"0"`
	SettledAt              int64  `json:"settledAt,omitempty" dc:"Settlement time as Unix timestamp in milliseconds, or zero" eg:"0"`
	WrittenOffAt           int64  `json:"writtenOffAt,omitempty" dc:"Write-off time as Unix timestamp in milliseconds, or zero" eg:"0"`
	Version                int64  `json:"version" dc:"Current optimistic statement version" eg:"2"`
	AvailableBalanceMinor  string `json:"availableBalanceMinor" dc:"Exact same-currency available customer balance" eg:"15000"`
	CanRepayFromBalance    bool   `json:"canRepayFromBalance" dc:"Whether a protected partial or full balance repayment is currently possible" eg:"true"`
	CreatedAt              int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1719792000000"`
	UpdatedAt              int64  `json:"updatedAt" dc:"Last lifecycle update as Unix timestamp in milliseconds" eg:"1720000000000"`
}

// CreditStatementLine is one immutable customer-visible statement line.
type CreditStatementLine struct {
	StatementLineNo string `json:"statementLineNo" dc:"Stable immutable statement-line number" eg:"CSL_01J2ABCDEF"`
	LineNumber      int    `json:"lineNumber" dc:"Stable line sequence" eg:"1"`
	LineTypeCode    string `json:"lineTypeCode" dc:"Stable statement-line type: principal, late_fee, adjustment, or refund" eg:"principal"`
	SourceTypeCode  string `json:"sourceTypeCode" dc:"Stable lowercase business source code supplied by the owning billing workflow" eg:"order"`
	SourceReference string `json:"sourceReference" dc:"Customer-visible business source reference" eg:"ORD_01J2ABCDEF"`
	AmountMinor     string `json:"amountMinor" dc:"Signed exact line amount in decimal minor units" eg:"20000"`
	CreatedAt       int64  `json:"createdAt" dc:"Immutable line creation time as Unix timestamp in milliseconds" eg:"1719792000000"`
}

// CreditRepayment is one safe immutable repayment summary.
type CreditRepayment struct {
	RepaymentNo            string `json:"repaymentNo" dc:"Stable immutable repayment number" eg:"CRY_01J2ABCDEF"`
	RepaymentTypeCode      string `json:"repaymentTypeCode" dc:"Stable repayment type: statement, prepayment, or bad_debt_recovery" eg:"statement"`
	FundingSourceTypeCode  string `json:"fundingSourceTypeCode" dc:"Confirmed funding source type: online_payment, customer_balance, manual_receipt, or migration" eg:"customer_balance"`
	AmountMinor            string `json:"amountMinor" dc:"Exact repayment amount in decimal minor units" eg:"10000"`
	AllocatedAmountMinor   string `json:"allocatedAmountMinor" dc:"Exact allocated repayment amount in decimal minor units" eg:"10000"`
	UnallocatedAmountMinor string `json:"unallocatedAmountMinor" dc:"Exact unallocated repayment amount in decimal minor units" eg:"0"`
	StatusCode             string `json:"statusCode" dc:"Current repayment lifecycle code: pending, confirmed, partially_allocated, allocated, refunded, or failed" eg:"allocated"`
	ConfirmedAt            int64  `json:"confirmedAt,omitempty" dc:"Funding confirmation time as Unix timestamp in milliseconds, or zero" eg:"1720000000000"`
	CreatedAt              int64  `json:"createdAt" dc:"Repayment creation time as Unix timestamp in milliseconds" eg:"1720000000000"`
}

// CreditStatementDetail combines one statement header with bounded lines and repayments.
type CreditStatementDetail struct {
	Statement           *CreditStatement       `json:"statement" dc:"Current-customer credit statement header" eg:"{}"`
	Lines               []*CreditStatementLine `json:"lines" dc:"At most 100 immutable statement lines in this page" eg:"[]"`
	NextLineCursor      string                 `json:"nextLineCursor" dc:"Encrypted next-line cursor, or empty" eg:""`
	Repayments          []*CreditRepayment     `json:"repayments" dc:"At most 100 safe repayment summaries" eg:"[]"`
	RepaymentsTruncated bool                   `json:"repaymentsTruncated" dc:"Whether older repayment summaries were omitted" eg:"false"`
}

// CreditActionProof carries a short-lived proof for one exact protected credit action.
type CreditActionProof struct {
	ProofToken string `json:"proofToken" dc:"Opaque session-, actor-, purpose-, and exact-preview-bound single-use proof" eg:"opaque-proof"`
	ExpiresAt  int64  `json:"expiresAt" dc:"Proof expiry as Unix timestamp in milliseconds" eg:"1720000000000"`
}

// CreditBalanceRepayment is the customer-safe committed balance repayment projection.
type CreditBalanceRepayment struct {
	RepaymentNo            string `json:"repaymentNo" dc:"Stable committed repayment number" eg:"CRY_01J2ABCDEF"`
	CreditAccountNo        string `json:"creditAccountNo" dc:"Owning credit account number" eg:"CRA_01J2ABCDEF"`
	CurrencyCode           string `json:"currencyCode" dc:"Exact repayment currency" eg:"CNY"`
	FundingSourceTypeCode  string `json:"fundingSourceTypeCode" dc:"Confirmed funding source; customer execution always returns customer_balance" eg:"customer_balance"`
	AmountMinor            string `json:"amountMinor" dc:"Exact repayment amount in decimal minor units" eg:"10000"`
	AllocatedAmountMinor   string `json:"allocatedAmountMinor" dc:"Exact allocated repayment amount in decimal minor units" eg:"10000"`
	UnallocatedAmountMinor string `json:"unallocatedAmountMinor" dc:"Exact unallocated repayment amount in decimal minor units" eg:"0"`
	StatusCode             string `json:"statusCode" dc:"Committed repayment lifecycle code: pending, confirmed, partially_allocated, allocated, refunded, or failed" eg:"allocated"`
}

// CreditBalanceRepaymentResult reports the atomic repayment, statement, and balance outcome.
type CreditBalanceRepaymentResult struct {
	Repayment                 *CreditBalanceRepayment `json:"repayment" dc:"Committed credit repayment without fabricated history timestamps" eg:"{}"`
	StatementStatusCode       string                  `json:"statementStatusCode" dc:"Current statement lifecycle code: open, partially_paid, paid, overdue, written_off, cancelled, or refunded" eg:"paid"`
	StatementOutstandingMinor string                  `json:"statementOutstandingMinor" dc:"Exact remaining statement amount in decimal minor units" eg:"0"`
	BalanceMinor              string                  `json:"balanceMinor" dc:"Exact resulting customer balance in decimal minor units" eg:"5000"`
	HeldBalanceMinor          string                  `json:"heldBalanceMinor" dc:"Exact still-held customer balance in decimal minor units" eg:"0"`
	AvailableBalanceMinor     string                  `json:"availableBalanceMinor" dc:"Exact resulting available customer balance in decimal minor units" eg:"5000"`
	Reused                    bool                    `json:"reused" dc:"Whether an exact repayment retry reused the committed result" eg:"false"`
}
