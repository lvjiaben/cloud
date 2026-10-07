package billing

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

// CreditDecision is an append-only staff decision on a credit application.
type CreditDecision string

const (
	CreditDecisionApprove CreditDecision = "approved"
	CreditDecisionReject  CreditDecision = "rejected"
)

// CreditPolicyInput appends one currency-local policy revision. IdempotencyKey
// is hashed into a stable business number and is never persisted verbatim.
type CreditPolicyInput struct {
	CurrencyCode             string
	MaximumLimitMinor        int64
	DualReviewThresholdMinor int64
	BillingDay               int
	RepaymentPeriodDays      int
	GracePeriodDays          int
	DailyLateFeeBasisPoints  int
	MaximumLateFeeMinor      int64
	ConfiguredBy             int
	IdempotencyKey           string
}

type CreditPolicy struct {
	PolicyNo                 string
	Version                  int64
	CurrencyCode             string
	MaximumLimitMinor        int64
	DualReviewThresholdMinor int64
	BillingDay               int
	RepaymentPeriodDays      int
	GracePeriodDays          int
	DailyLateFeeBasisPoints  int
	MaximumLateFeeMinor      int64
	ConfiguredBy             int
	ContentDigest            string
	CreatedAt                time.Time
	Reused                   bool
}

// CreditApplicationInput captures a bounded risk reference rather than raw
// evidence. Customer and staff entry points must supply their authenticated
// actor type and identifier.
type CreditApplicationInput struct {
	CustomerNo            string
	CurrencyCode          string
	RequestedLimitMinor   int64
	RequestedValidFrom    time.Time
	RequestedValidUntil   time.Time
	RiskEvidenceReference string
	RiskEvidenceDigest    string
	RequestActorType      string
	RequestedBy           int
	IdempotencyKey        string
}

type CreditReviewInput struct {
	ApplicationNo  string
	Decision       CreditDecision
	ReviewerID     int
	CommentDigest  string
	EvidenceDigest string
}

// CreditReserveInput atomically turns an approved same-currency limit into an
// unbilled exposure. SourceType and SourceReference are the business-level
// idempotency identity; a replay may not change amount or evidence.
type CreditReserveInput struct {
	CustomerNo           string
	CurrencyCode         string
	AmountMinor          int64
	SourceType           string
	SourceReference      string
	OrderNo              string
	InvoiceNo            string
	SourceSnapshotDigest string
	ActorType            string
	ActorID              int
}

// CreditReleaseInput fully releases one still-unbilled exposure. Partial and
// post-statement refund allocation are intentionally owned by the refund flow.
type CreditReleaseInput struct {
	ExposureNo      string
	SourceReference string
	EvidenceDigest  string
	ActorType       string
	ActorID         int
}

// CreditRefundComponent identifies which part of one credit purchase is being
// reversed. The order is fixed: unbilled, billed but unpaid, then repaid.
type CreditRefundComponent string

// CreditRefundDestination identifies the customer-approved destination for
// money that had already repaid a credit purchase.
type CreditRefundDestination string

const (
	// CreditRefundUnbilled releases principal that has not entered a statement.
	CreditRefundUnbilled CreditRefundComponent = "unbilled"
	// CreditRefundBilled reduces billed principal that has not been repaid.
	CreditRefundBilled CreditRefundComponent = "billed"
	// CreditRefundRepaid returns money through the original repayment source.
	CreditRefundRepaid CreditRefundComponent = "repaid"

	// CreditRefundDestinationOriginalSource returns money through the original owner.
	CreditRefundDestinationOriginalSource CreditRefundDestination = "original_source"
	// CreditRefundDestinationBalance credits the customer's same-currency balance.
	CreditRefundDestinationBalance CreditRefundDestination = "same_currency_balance"
)

// CreditRefundFundingSource is one bounded original repayment allocation that
// must receive a real refund before the repaid component can be committed.
type CreditRefundFundingSource struct {
	RepaymentAllocationNo  string
	RepaymentNo            string
	AllocationType         string
	FundingSourceType      string
	FundingSourceReference string
	AmountMinor            int64
}

// CreditRefundPlan freezes the current component order and exact original
// funding sources. PlanDigest must be supplied unchanged when committing.
type CreditRefundPlan struct {
	ExposureNo           string
	CreditAccountNo      string
	CustomerNo           string
	CurrencyCode         string
	StatementNo          string
	AmountMinor          int64
	UnbilledReleaseMinor int64
	BilledReductionMinor int64
	RepaidRefundMinor    int64
	FundingSources       []CreditRefundFundingSource
	PlanDigest           string
}

// CreditRefundFundingResult proves that one planned repaid amount has already
// been returned through its original funding destination.
type CreditRefundFundingResult struct {
	RepaymentAllocationNo string
	AmountMinor           int64
	RefundSourceReference string
	EvidenceDigest        string
	Destination           CreditRefundDestination
}

// CreditRefundInput commits one exact plan. SourceReference is the stable
// business idempotency identity and raw request keys must not be persisted.
type CreditRefundInput struct {
	ExposureNo      string
	AmountMinor     int64
	PlanDigest      string
	SourceReference string
	EvidenceDigest  string
	ActorType       string
	ActorID         int
	FundingResults  []CreditRefundFundingResult
}

// CreditRefund is the authoritative credit projection after one committed
// refund. Reused is true only for an exact idempotent replay.
type CreditRefund struct {
	ExposureNo            string
	CreditAccountNo       string
	CustomerNo            string
	CurrencyCode          string
	StatementNo           string
	StatementStatusCode   string
	AmountMinor           int64
	UnbilledReleaseMinor  int64
	BilledReductionMinor  int64
	RepaidRefundMinor     int64
	ExposureStatusCode    string
	AccountAvailableMinor int64
	AccountUnbilledMinor  int64
	AccountBilledMinor    int64
	AccountVersion        int64
	Reused                bool
}

type CreditExposure struct {
	ExposureNo               string
	CreditAccountNo          string
	CustomerNo               string
	CurrencyCode             string
	SourceType               string
	SourceReference          string
	OriginalPrincipalMinor   int64
	UnbilledPrincipalMinor   int64
	BilledPrincipalMinor     int64
	RepaidPrincipalMinor     int64
	ReleasedPrincipalMinor   int64
	WrittenOffPrincipalMinor int64
	StatusCode               string
	AccountTotalLimitMinor   int64
	AccountAvailableMinor    int64
	AccountUnbilledMinor     int64
	AccountBilledMinor       int64
	AccountVersion           int64
	Reused                   bool
}

type CreditStatementIssueInput struct {
	CreditAccountNo string
	PeriodStartedAt time.Time
	PeriodEndedAt   time.Time
	IdempotencyKey  string
}

type CreditStatement struct {
	StatementNo           string
	CreditAccountNo       string
	CustomerNo            string
	CurrencyCode          string
	TermsNo               string
	TermsVersion          int64
	PeriodStartedAt       time.Time
	PeriodEndedAt         time.Time
	LineCount             int
	PrincipalMinor        int64
	TotalAmountMinor      int64
	OutstandingMinor      int64
	StatusCode            string
	IssuedAt              time.Time
	PaymentDueAt          time.Time
	AccountAvailableMinor int64
	AccountUnbilledMinor  int64
	AccountBilledMinor    int64
	AccountVersion        int64
	Reused                bool
}

// CreditStatementBatch reports one bounded merchant-timezone billing run.
type CreditStatementBatch struct {
	Selected int
	Issued   int
	Reused   int
	Failures int
}

type CreditPrepaymentInput struct {
	ExposureNo                 string
	FundingSourceType          string
	FundingSourceReference     string
	LedgerTransactionNo        string
	AmountMinor                int64
	SourceSnapshotDigest       string
	ConfirmationEvidenceDigest string
}

type CreditStatementRepaymentInput struct {
	StatementNo                string
	FundingSourceType          string
	FundingSourceReference     string
	LedgerTransactionNo        string
	AmountMinor                int64
	SourceSnapshotDigest       string
	ConfirmationEvidenceDigest string
}

// CreditBalanceRepaymentInput spends only the statement customer's exact
// same-currency balance. AmountMinor may be a bounded partial repayment; the
// service derives customer, currency, and receivable ownership from the
// locked statement rather than trusting caller-supplied identities.
type CreditBalanceRepaymentInput struct {
	StatementNo                string
	AmountMinor                int64
	SourceReference            string
	SourceSnapshotDigest       string
	ConfirmationEvidenceDigest string
}

// CreditBalanceRepayment is the atomic balance and credit projection after
// posting the dedicated double-entry transaction and allocating repayment.
type CreditBalanceRepayment struct {
	Repayment             *CreditRepayment
	BalanceMinor          int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	Reused                bool
}

type CreditRepayment struct {
	RepaymentNo               string
	CreditAccountNo           string
	CustomerNo                string
	CurrencyCode              string
	RepaymentType             string
	FundingSourceType         string
	FundingSourceReference    string
	LedgerTransactionNo       string
	AmountMinor               int64
	AllocatedAmountMinor      int64
	UnallocatedAmountMinor    int64
	StatusCode                string
	ExposureNo                string
	ExposureStatusCode        string
	StatementNo               string
	StatementStatusCode       string
	StatementOutstandingMinor int64
	AccountStatusCode         string
	AccountAvailableMinor     int64
	AccountUnbilledMinor      int64
	AccountBilledMinor        int64
	AccountDelinquentMinor    int64
	AccountWrittenOffMinor    int64
	AccountVersion            int64
	Reused                    bool
}

// CreditDelinquency is the bounded projection of one overdue statement case.
// ServiceActionTaskNo identifies a durable internal task, never a provider task.
type CreditDelinquency struct {
	CaseNo                   string
	StatementNo              string
	CreditAccountNo          string
	CustomerNo               string
	CurrencyCode             string
	StatusCode               string
	PrincipalMinor           int64
	LateFeeMinor             int64
	OutstandingAmountMinor   int64
	OpenedAt                 time.Time
	GraceUntil               time.Time
	FrozenAt                 *time.Time
	ServiceActionRequestedAt *time.Time
	ResolvedAt               *time.Time
	AccountStatusCode        string
	AccountDelinquentMinor   int64
	AccountVersion           int64
	ServiceActionTaskNo      string
	Reused                   bool
}

// CreditWriteoffInput consumes a previously approved two-person decision and
// its already-posted ledger evidence. The HTTP approval workflow remains the
// owner of staff permissions, data scope, reauthentication, and audit.
type CreditWriteoffInput struct {
	WriteoffNo               string
	StatementNo              string
	ExpectedOutstandingMinor int64
	LedgerTransactionNo      string
	ReasonCode               string
	RequestedBy              int
	ReviewedBy               int
	ApprovalEvidenceDigest   string
}

type CreditWriteoff struct {
	WriteoffNo                string
	StatementNo               string
	CreditAccountNo           string
	CustomerNo                string
	CurrencyCode              string
	AmountMinor               int64
	LedgerTransactionNo       string
	ReasonCode                string
	RequestedBy               int
	ReviewedBy                int
	PostedAt                  time.Time
	StatementStatusCode       string
	StatementOutstandingMinor int64
	AccountStatusCode         string
	AccountAvailableMinor     int64
	AccountBilledMinor        int64
	AccountDelinquentMinor    int64
	AccountWrittenOffMinor    int64
	AccountVersion            int64
	Reused                    bool
}

// CreditRestorationInput consumes a reviewed manual decision. The caller owns
// exact staff permission, data-scope, reauthentication, and business audit.
type CreditRestorationInput struct {
	CreditAccountNo string
	ExpectedVersion int64
	SourceReference string
	ReasonCode      string
	EvidenceDigest  string
	ActorID         int
}

type CreditRestoration struct {
	CreditAccountNo        string
	CustomerNo             string
	CurrencyCode           string
	PreviousStatusCode     string
	RestoredStatusCode     string
	CurrentStatusCode      string
	AccountAvailableMinor  int64
	AccountDelinquentMinor int64
	AccountWrittenOffMinor int64
	AccountVersion         int64
	SourceReference        string
	OccurredAt             time.Time
	Reused                 bool
}

type CreditReconciliationItem struct {
	CreditAccountNo              string
	CustomerNo                   string
	CurrencyCode                 string
	AccountVersion               int64
	DiscrepancyCodes             []string
	ProjectedUnbilledMinor       int64
	ExposureUnbilledMinor        int64
	ProjectedBilledMinor         int64
	ExposureBilledMinor          int64
	ProjectedDelinquentMinor     int64
	CaseDelinquentMinor          int64
	ProjectedWrittenOffMinor     int64
	ExposureWrittenOffMinor      int64
	LatestMovementAccountVersion int64
}

type CreditReconciliationBatch struct {
	Items        []CreditReconciliationItem
	ScannedCount int
	NextCursor   string
	HasMore      bool
	AlertTaskNo  string
	Reused       bool
}

type CreditApplication struct {
	ApplicationNo         string
	CustomerNo            string
	CurrencyCode          string
	PolicyNo              string
	PolicyVersion         int64
	RequestedLimitMinor   int64
	RequestedValidFrom    time.Time
	RequestedValidUntil   time.Time
	RequiredApprovalCount int
	StatusCode            string
	RequestActorType      string
	RequestedBy           int
	ReviewCount           int
	CreditAccountNo       string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Reused                bool
}

// CreditApprovalService owns policy revision, application, review, and atomic
// account activation. HTTP callers remain responsible for exact permissions,
// direct identity, data scope, and recent-authentication requirements.
type CreditApprovalService interface {
	ConfigureCreditPolicy(context.Context, CreditPolicyInput) (*CreditPolicy, error)
	SubmitCreditApplication(context.Context, CreditApplicationInput) (*CreditApplication, error)
	ReviewCreditApplication(context.Context, CreditReviewInput) (*CreditApplication, error)
}

// CreditUsageService owns the serialized same-currency limit projection. It
// never reads or writes customer balance, gift, or commission accounts.
type CreditUsageService interface {
	ReserveCredit(context.Context, CreditReserveInput) (*CreditExposure, error)
	ReleaseUnbilledCredit(context.Context, CreditReleaseInput) (*CreditExposure, error)
	// PreviewCreditRefund derives the exact current credit and original-funding
	// split without changing state. Written-off exposure fails closed.
	PreviewCreditRefund(context.Context, string, int64) (*CreditRefundPlan, error)
	// ApplyConfirmedCreditRefund atomically commits an unchanged plan after all
	// required original-source refunds have authoritative evidence.
	ApplyConfirmedCreditRefund(context.Context, CreditRefundInput) (*CreditRefund, error)
	IssueCreditStatement(context.Context, CreditStatementIssueInput) (*CreditStatement, error)
	// RunCreditStatementBatch finds only currently due accounts in one bounded
	// set query; each selected account is then issued through the serialized,
	// idempotent statement boundary.
	RunCreditStatementBatch(context.Context, int) (*CreditStatementBatch, error)
	// RepayCreditStatementFromBalance atomically debits only the owning
	// customer's same-currency available balance and credits merchant
	// receivables before applying the trusted credit repayment.
	RepayCreditStatementFromBalance(context.Context, CreditBalanceRepaymentInput) (*CreditBalanceRepayment, error)
	ApplyConfirmedCreditPrepayment(context.Context, CreditPrepaymentInput) (*CreditRepayment, error)
	ApplyConfirmedCreditStatementRepayment(context.Context, CreditStatementRepaymentInput) (*CreditRepayment, error)
}

// CreditDelinquencyService owns overdue derivation and durable service-action
// requests. It never calls a Provider or mutates a service runtime state.
type CreditDelinquencyService interface {
	OpenDueCreditStatement(context.Context, string) (*CreditDelinquency, error)
	AccrueCreditLateFee(context.Context, string) (*CreditDelinquency, error)
	EscalateCreditDelinquency(context.Context, string) (*CreditDelinquency, error)
}

// CreditBadDebtService owns the accounting transition after an independent
// two-person approval and trusted ledger posting have already completed.
type CreditBadDebtService interface {
	WriteOffCreditStatement(context.Context, CreditWriteoffInput) (*CreditWriteoff, error)
	ApplyConfirmedBadDebtRecovery(context.Context, CreditStatementRepaymentInput) (*CreditRepayment, error)
	RestoreCreditAccount(context.Context, CreditRestorationInput) (*CreditRestoration, error)
}

type CreditReconciliationService interface {
	ScanCreditReconciliation(
		context.Context, string, int,
	) (*CreditReconciliationBatch, error)
}

var _ CreditApprovalService = (*serviceImpl)(nil)
var _ CreditUsageService = (*serviceImpl)(nil)
var _ CreditDelinquencyService = (*creditDelinquencyServiceImpl)(nil)
var _ CreditBadDebtService = (*serviceImpl)(nil)
var _ CreditReconciliationService = (*creditReconciliationServiceImpl)(nil)

func NewCreditApprovalService(database gdb.DB) (CreditApprovalService, error) {
	return newBalanceService(database, rand.Reader)
}

func NewCreditUsageService(database gdb.DB) (CreditUsageService, error) {
	return newBalanceService(database, rand.Reader)
}

func NewCreditBadDebtService(database gdb.DB) (CreditBadDebtService, error) {
	return newBalanceService(database, rand.Reader)
}

func NewCreditReconciliationService(
	database gdb.DB,
	tasks asynctask.Service,
) (CreditReconciliationService, error) {
	billingService, err := newBalanceService(database, rand.Reader)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return nil, gerror.New("credit reconciliation service requires an asynchronous task service")
	}
	return &creditReconciliationServiceImpl{serviceImpl: billingService, tasks: tasks}, nil
}

// NewCreditDelinquencyService explicitly receives the shared durable task
// owner used to separate credit state changes from later service operations.
func NewCreditDelinquencyService(
	database gdb.DB,
	tasks asynctask.Service,
) (CreditDelinquencyService, error) {
	billingService, err := newBalanceService(database, rand.Reader)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return nil, gerror.New("credit delinquency service requires an asynchronous task service")
	}
	return &creditDelinquencyServiceImpl{serviceImpl: billingService, tasks: tasks}, nil
}
