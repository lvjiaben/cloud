// Package billing owns the append-only monetary ledger and customer balance
// operations. It deliberately exposes semantic operations instead of a
// caller-defined journal-entry API.
package billing

import (
	"context"
	"crypto/rand"
	"io"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	AccountTypeCustomerBalance    = "customer_balance"
	AccountTypeCustomerCredit     = "customer_credit"
	AccountTypeCustomerGift       = "customer_gift"
	AccountTypeCustomerCommission = "customer_commission"

	maximumAmountMinor int64 = 9_000_000_000_000_000
)

// InvoiceStatus is the persisted invoice lifecycle state. Callers cannot set
// it directly; every change is derived from a semantic invoice operation.
type InvoiceStatus string

const (
	InvoiceStatusDraft             InvoiceStatus = "draft"
	InvoiceStatusPendingPayment    InvoiceStatus = "pending_payment"
	InvoiceStatusPartiallyPaid     InvoiceStatus = "partially_paid"
	InvoiceStatusPaid              InvoiceStatus = "paid"
	InvoiceStatusOverdue           InvoiceStatus = "overdue"
	InvoiceStatusCancelled         InvoiceStatus = "cancelled"
	InvoiceStatusPartiallyRefunded InvoiceStatus = "partially_refunded"
	InvoiceStatusRefunded          InvoiceStatus = "refunded"
	InvoiceStatusBadDebt           InvoiceStatus = "bad_debt"
	InvoiceStatusCreditSettled     InvoiceStatus = "credit_settled"
)

// InvoiceTransformType distinguishes an atomic merge from a whole-item split.
type InvoiceTransformType string

const (
	InvoiceTransformMerge InvoiceTransformType = "merge"
	InvoiceTransformSplit InvoiceTransformType = "split"
)

// FinancialAdjustmentStatus is the reviewed intent lifecycle. Only a posted
// request has changed ledger or invoice facts.
type FinancialAdjustmentStatus string

const (
	AdjustmentStatusPendingReview FinancialAdjustmentStatus = "pending_review"
	AdjustmentStatusRejected      FinancialAdjustmentStatus = "rejected"
	AdjustmentStatusPosted        FinancialAdjustmentStatus = "posted"
)

// AdjustmentDecision is the append-only authorized decision.
type AdjustmentDecision string

const (
	AdjustmentDecisionApprove AdjustmentDecision = "approved"
	AdjustmentDecisionReject  AdjustmentDecision = "rejected"
)

// PaymentSourceType identifies the trusted owner of a payment allocation.
type PaymentSourceType string

const (
	PaymentSourceChannel       PaymentSourceType = "channel_payment"
	PaymentSourceBalance       PaymentSourceType = "balance"
	PaymentSourceCredit        PaymentSourceType = "credit"
	PaymentSourceManualReceipt PaymentSourceType = "manual_receipt"
	PaymentSourceMigration     PaymentSourceType = "migration"
)

// FinancialSettlementBasis identifies the one authoritative reason an order
// has become financially eligible for fulfillment.
type FinancialSettlementBasis string

const (
	SettlementBasisFundedInvoice FinancialSettlementBasis = "funded_invoice"
	SettlementBasisCredit        FinancialSettlementBasis = "credit"
	SettlementBasisZeroAmount    FinancialSettlementBasis = "zero_amount"
	SettlementBasisFullDiscount  FinancialSettlementBasis = "full_discount"
	SettlementBasisTrial         FinancialSettlementBasis = "trial"
)

// FinancialSettlementSourceType identifies one authoritative settlement
// evidence owner without exposing that owner's private payload.
type FinancialSettlementSourceType string

const (
	FinancialSettlementSourceChannelPayment FinancialSettlementSourceType = "channel_payment"
	FinancialSettlementSourceBalance        FinancialSettlementSourceType = "balance"
	FinancialSettlementSourceManualReceipt  FinancialSettlementSourceType = "manual_receipt"
	FinancialSettlementSourceCredit         FinancialSettlementSourceType = "credit"
	FinancialSettlementSourceZeroAmount     FinancialSettlementSourceType = "zero_amount"
	FinancialSettlementSourceFullDiscount   FinancialSettlementSourceType = "full_discount"
	FinancialSettlementSourceTrial          FinancialSettlementSourceType = "trial"
)

// FinancialSettlementSource is one bounded immutable source that contributes
// to a financially settled order. It deliberately contains no payment secret,
// identity material, or provider payload.
type FinancialSettlementSource struct {
	SettlementSourceNo  string
	SourceType          FinancialSettlementSourceType
	SourceReference     string
	PaymentAllocationNo string
	CreditExposureNo    string
	AmountMinor         int64
	EvidenceDigest      string
}

// FinancialSettlement is the stable internal projection of one immutable
// settlement fact and all of its bounded source evidence.
type FinancialSettlement struct {
	SettlementNo           string
	OrderNo                string
	InvoiceNo              string
	CustomerNo             string
	CurrencyCode           string
	Basis                  FinancialSettlementBasis
	TotalAmountMinor       int64
	PaidAmountMinor        int64
	CreditAmountMinor      int64
	IdentityEvidenceDigest string
	SourceSnapshotDigest   string
	SettledAt              time.Time
	Sources                []FinancialSettlementSource
	Reused                 bool
}

// CreditOrderSettlementInput identifies only the order and the authenticated
// actor. Customer, invoice, currency, amount, and credit evidence are derived
// from locked owner data and cannot be supplied by the caller.
type CreditOrderSettlementInput struct {
	OrderNo   string
	ActorType string
	ActorID   int
}

// CreditReservationService is the narrow credit-owner dependency required by
// settlement. The broader credit lifecycle remains outside this coordinator.
type CreditReservationService interface {
	ReserveCredit(context.Context, CreditReserveInput) (*CreditExposure, error)
}

// RefundSourceType identifies the trusted owner of a completed refund.
type RefundSourceType string

const (
	RefundSourceChannel       RefundSourceType = "channel_refund"
	RefundSourceBalanceCredit RefundSourceType = "balance_credit"
	RefundSourceCreditRelease RefundSourceType = "credit_release"
	RefundSourceManual        RefundSourceType = "manual"
	RefundSourceMigration     RefundSourceType = "migration"
)

var (
	currencyPattern        = regexp.MustCompile(`^[A-Z]{3}$`)
	sourceTypePattern      = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,31}$`)
	canonicalDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ConfirmedRechargeInput records money only after a trusted payment owner has
// verified the channel event. This service does not contact or trust channels.
type ConfirmedRechargeInput struct {
	CustomerNo           string
	CurrencyCode         string
	AmountMinor          int64
	SourceReference      string
	SourceSnapshotDigest string
}

// BalanceConsumptionInput spends available balance for one immutable business
// fact. Funds reserved by active holds are not available to this operation.
type BalanceConsumptionInput struct {
	CustomerNo           string
	CurrencyCode         string
	AmountMinor          int64
	SourceReference      string
	SourceSnapshotDigest string
}

// ReversalInput appends a full inverse of one original transaction.
type ReversalInput struct {
	OriginalTransactionNo string
	SourceReference       string
	SourceSnapshotDigest  string
}

// HoldCreateInput reserves, but does not spend, customer balance.
type HoldCreateInput struct {
	CustomerNo           string
	CurrencyCode         string
	AmountMinor          int64
	SourceType           string
	SourceReference      string
	SourceSnapshotDigest string
	ExpiresAt            time.Time
}

// HoldConsumeInput spends all or part of one existing reservation.
type HoldConsumeInput struct {
	HoldNo               string
	InvoiceNo            string
	AmountMinor          int64
	SourceReference      string
	SourceSnapshotDigest string
}

// Posting is the stable result of one append-only monetary transaction.
type Posting struct {
	TransactionNo         string
	TransactionType       string
	CurrencyCode          string
	AmountMinor           int64
	CustomerNo            string
	BalanceMinor          int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	Reused                bool
}

// Hold is the stable projection of one customer-balance reservation.
type Hold struct {
	HoldNo               string
	CustomerNo           string
	CurrencyCode         string
	AmountMinor          int64
	ConsumedAmountMinor  int64
	ReleasedAmountMinor  int64
	RemainingAmountMinor int64
	Status               string
	ExpiresAt            time.Time
	Reused               bool
}

// AccountProjection is rebuilt from immutable entries and compared with the
// stored snapshot chain. Rebuild never mutates historical snapshots.
type AccountProjection struct {
	AccountNo             string
	CustomerNo            string
	AccountType           string
	CurrencyCode          string
	NormalBalance         string
	TransactionCount      int
	EntryCount            int
	RebuiltBalanceMinor   int64
	SnapshotBalanceMinor  int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	ContentDigest         string
	Consistent            bool
}

// InvoicePaymentInput allocates one already-posted immutable ledger
// transaction to an invoice or one invoice item.
type InvoicePaymentInput struct {
	InvoiceNo           string
	InvoiceItemNo       string
	SourceType          PaymentSourceType
	SourceReference     string
	LedgerTransactionNo string
	AmountMinor         int64
}

// InvoiceBalancePaymentInput settles the exact current outstanding amount of
// one invoice from its owning customer's same-currency available balance.
// Customer, currency, and amount are deliberately derived from locked invoice
// facts instead of being accepted from the caller.
type InvoiceBalancePaymentInput struct {
	InvoiceNo            string
	SourceReference      string
	SourceSnapshotDigest string
}

// InvoicePaymentReversalInput reverses part or all of one payment allocation
// using an immutable reversal transaction.
type InvoicePaymentReversalInput struct {
	OriginalAllocationNo string
	SourceReference      string
	LedgerTransactionNo  string
	AmountMinor          int64
}

// InvoiceRefundInput allocates one completed refund to its original payment
// allocation and mandatory invoice item.
type InvoiceRefundInput struct {
	InvoiceNo           string
	InvoiceItemNo       string
	PaymentAllocationNo string
	SourceType          RefundSourceType
	SourceReference     string
	LedgerTransactionNo string
	AmountMinor         int64
}

// InvoiceRefundReversalInput reverses part or all of one refund allocation
// using an immutable reversal transaction.
type InvoiceRefundReversalInput struct {
	OriginalRefundAllocationNo string
	SourceReference            string
	LedgerTransactionNo        string
	AmountMinor                int64
}

// InvoiceBadDebtInput recognizes an overdue receivable only after an
// immutable write-off transaction proves the exact outstanding amount.
type InvoiceBadDebtInput struct {
	InvoiceNo           string
	LedgerTransactionNo string
}

// InvoiceMergeInput merges two or more untouched payable invoices. The raw
// idempotency key is hashed in process and is never persisted.
type InvoiceMergeInput struct {
	InvoiceNos     []string
	IdempotencyKey string
	RequestedBy    int
}

// InvoiceSplitGroup assigns whole source items to one destination invoice.
// A source item must appear in exactly one group.
type InvoiceSplitGroup struct {
	InvoiceItemNos []string
}

// InvoiceSplitInput splits every whole item of one untouched payable invoice
// into two or more non-empty destination groups.
type InvoiceSplitInput struct {
	InvoiceNo      string
	Groups         []InvoiceSplitGroup
	IdempotencyKey string
	RequestedBy    int
}

// AdjustmentPolicyInput appends the next currency-local high-value review
// threshold. Currencies are never compared through foreign exchange.
type AdjustmentPolicyInput struct {
	CurrencyCode         string
	ReviewThresholdMinor int64
	ConfiguredBy         int
}

// ManualReceiptInput records verified offline money. Exactly one target is
// required: InvoiceNo, CreditStatementNo, or a named customer's same-currency
// balance through CustomerNo and CurrencyCode.
type ManualReceiptInput struct {
	InvoiceNo                 string
	CreditStatementNo         string
	CustomerNo                string
	CurrencyCode              string
	AmountMinor               int64
	ReasonCode                string
	ProofReference            string
	ReceivingAccountReference string
	BusinessPurposeCode       string
	EvidenceDigest            string
	IdempotencyKey            string
	RequestedBy               int
}

// InvoiceWaiverInput reduces one invoice's outstanding amount without
// reporting the waived value as customer payment.
type InvoiceWaiverInput struct {
	InvoiceNo      string
	AmountMinor    int64
	ReasonCode     string
	EvidenceDigest string
	IdempotencyKey string
	RequestedBy    int
}

// MarketingGrantAdjustmentInput requests one reviewed posting or reversal for
// an exact marketing value grant. The grant owner supplies immutable evidence;
// billing owns the customer-gift and merchant-expense ledger movement.
type MarketingGrantAdjustmentInput struct {
	GrantNo        string
	CustomerNo     string
	CurrencyCode   string
	AmountMinor    int64
	ReasonCode     string
	EvidenceDigest string
	IdempotencyKey string
	RequestedBy    int
}

// AdjustmentReviewInput appends one authorized decision. The requester may also process their historic pending request.
type AdjustmentReviewInput struct {
	AdjustmentNo   string
	Decision       AdjustmentDecision
	ReviewerID     int
	CommentDigest  string
	EvidenceDigest string
}

// Invoice is the bounded lifecycle projection returned to trusted internal
// owners. It contains no payment-channel credentials or mutable ledger rows.
type Invoice struct {
	InvoiceNo              string
	OrderNo                string
	CustomerNo             string
	CurrencyCode           string
	Status                 InvoiceStatus
	TotalAmountMinor       int64
	PaidAmountMinor        int64
	WaivedAmountMinor      int64
	CreditAmountMinor      int64
	OutstandingAmountMinor int64
	RefundableAmountMinor  int64
	RefundedAmountMinor    int64
	PaymentDueAt           time.Time
	IssuedAt               *time.Time
	SettledAt              *time.Time
	CancelledAt            *time.Time
	Version                int64
	AllocationNo           string
	Reused                 bool
	BecamePaid             bool
}

// InvoiceBalancePayment is the atomic result of one same-currency balance
// posting and its matching invoice allocation.
type InvoiceBalancePayment struct {
	Posting *Posting
	Invoice *Invoice
	Reused  bool
}

// AdjustmentPolicy is one immutable threshold revision.
type AdjustmentPolicy struct {
	PolicyNo             string
	CurrencyCode         string
	ReviewThresholdMinor int64
	Version              int64
	ConfiguredBy         int
	Reused               bool
}

// FinancialAdjustment is the bounded projection of a receipt or waiver.
type FinancialAdjustment struct {
	AdjustmentNo        string
	AdjustmentType      string
	Status              FinancialAdjustmentStatus
	CustomerNo          string
	InvoiceNo           string
	CreditStatementNo   string
	CurrencyCode        string
	AmountMinor         int64
	ApprovalRequired    bool
	RequestedBy         int
	LedgerTransactionNo string
	AllocationNo        string
	Invoice             *Invoice
	CreditRepayment     *CreditRepayment
	Balance             *Posting
	Reused              bool
}

// InvoiceTransform is the bounded immutable merge or split projection.
type InvoiceTransform struct {
	TransformNo         string
	Type                InvoiceTransformType
	CustomerNo          string
	CurrencyCode        string
	SourceInvoiceNos    []string
	DestinationInvoices []*Invoice
	Reused              bool
}

// BalanceService owns same-currency balance operations and immutable ledger
// projection verification. It intentionally has no arbitrary-entry method.
type BalanceService interface {
	PostConfirmedRecharge(context.Context, ConfirmedRechargeInput) (*Posting, error)
	ConsumeAvailableBalance(context.Context, BalanceConsumptionInput) (*Posting, error)
	ReverseTransaction(context.Context, ReversalInput) (*Posting, error)
	CreateHold(context.Context, HoldCreateInput) (*Hold, error)
	ConsumeHold(context.Context, HoldConsumeInput) (*Posting, error)
	ReleaseHold(context.Context, string) (*Hold, error)
	ExpireHold(context.Context, string) (*Hold, error)
	RebuildCustomerAccountProjection(context.Context, string, string, string) (*AccountProjection, error)
}

// InvoiceService owns invoice lifecycle and immutable allocation aggregation.
// It is an internal authority boundary: HTTP and plugin callers must perform
// their own permission, ownership, approval, and payment-evidence checks before
// invoking these semantic operations.
type InvoiceService interface {
	// IssueDraft moves one untouched draft to a payable or already-overdue state.
	IssueDraft(context.Context, string) (*Invoice, error)
	// MarkOverdue derives overdue state from the database clock and due date.
	MarkOverdue(context.Context, string) (*Invoice, error)
	// CancelUnpaid cancels only an invoice with no effective financial activity.
	CancelUnpaid(context.Context, string) (*Invoice, error)
	// RecognizeBadDebt requires an exact immutable write-off transaction.
	RecognizeBadDebt(context.Context, InvoiceBadDebtInput) (*Invoice, error)
	// ApplyPayment appends and aggregates one trusted payment allocation.
	ApplyPayment(context.Context, InvoicePaymentInput) (*Invoice, error)
	// PayFromBalance atomically posts the exact outstanding amount and allocates it.
	PayFromBalance(context.Context, InvoiceBalancePaymentInput) (*InvoiceBalancePayment, error)
	// ReversePayment appends a payment-allocation reversal without mutating history.
	ReversePayment(context.Context, InvoicePaymentReversalInput) (*Invoice, error)
	// ApplyRefund appends an item- and payment-bound completed refund allocation.
	ApplyRefund(context.Context, InvoiceRefundInput) (*Invoice, error)
	// ReverseRefund appends a refund-allocation reversal without mutating history.
	ReverseRefund(context.Context, InvoiceRefundReversalInput) (*Invoice, error)
}

// InvoiceTransformService owns atomic same-customer, same-currency invoice
// merge and whole-item split. Callers must enforce direct-staff authority,
// exact permissions, the default merchant, and an expressible data scope.
type InvoiceTransformService interface {
	MergeInvoices(context.Context, InvoiceMergeInput) (*InvoiceTransform, error)
	SplitInvoice(context.Context, InvoiceSplitInput) (*InvoiceTransform, error)
}

// CustomerBalanceAdjustmentInput records a gift credit or an available-cash
// debit through the existing reviewed, immutable adjustment lifecycle.
type CustomerBalanceAdjustmentInput struct {
	CustomerNo     string
	CurrencyCode   string
	AmountMinor    int64
	Deduct         bool
	ReasonCode     string
	EvidenceDigest string
	IdempotencyKey string
	RequestedBy    int
}

// FinancialAdjustmentService owns reviewed manual receipts and invoice
// waivers. Callers must enforce direct-staff identity, exact create/review/
// policy permissions, merchant scope, and recent re-authentication.
type FinancialAdjustmentService interface {
	// CreateCustomerBalanceAdjustment directly posts an authorized adjustment;
	// debits recheck locked available cash and never spend frozen funds.
	CreateCustomerBalanceAdjustment(context.Context, CustomerBalanceAdjustmentInput) (*FinancialAdjustment, error)
	ConfigureApprovalPolicy(context.Context, AdjustmentPolicyInput) (*AdjustmentPolicy, error)
	CreateManualReceipt(context.Context, ManualReceiptInput) (*FinancialAdjustment, error)
	CreateInvoiceWaiver(context.Context, InvoiceWaiverInput) (*FinancialAdjustment, error)
	// CreateMarketingGrantPosting directly posts a permission-authorized
	// customer-gift credit and never changes the marketing entitlement itself.
	CreateMarketingGrantPosting(context.Context, MarketingGrantAdjustmentInput) (*FinancialAdjustment, error)
	// CreateMarketingGrantReversal directly posts a permission-authorized
	// reversal of an exact unused marketing grant amount.
	CreateMarketingGrantReversal(context.Context, MarketingGrantAdjustmentInput) (*FinancialAdjustment, error)
	ReviewAdjustment(context.Context, AdjustmentReviewInput) (*FinancialAdjustment, error)
}

// FinancialSettlementService is the only billing authority that can publish
// an order financially-settled fact. Fulfillment remains a downstream owner.
type FinancialSettlementService interface {
	// SettleOrder revalidates the locked order's local financial and identity
	// evidence. It returns an existing immutable fact on safe replay, returns a
	// business error when the order is ineligible or inconsistent, and never
	// advances fulfillment or resource state.
	SettleOrder(context.Context, string) (*FinancialSettlement, error)
	// SettleOrderWithCredit atomically reserves an approved same-currency
	// credit limit and publishes the one settlement fact. Any failure rolls
	// back the limit, exposure, movement, invoice transition, and event.
	SettleOrderWithCredit(context.Context, CreditOrderSettlementInput) (*FinancialSettlement, error)
}

// SettlementEvidenceReader exposes verified, already committed settlement
// evidence to local consumers. It cannot create a settlement or move money.
type SettlementEvidenceReader interface {
	// GetSettledOrderEvidence verifies the committed order's header, bounded
	// funding sources and unique outbox event. Missing/inconsistent evidence
	// returns a billing error; it never creates facts, moves money or changes
	// identity permissions. This is an internal worker contract, not an API.
	GetSettledOrderEvidence(context.Context, string) (*FinancialSettlement, error)
}

type settlementEvidenceReader struct{ database gdb.DB }

// NewSettlementEvidenceReader shares the billing owner's immutable evidence
// verifier, without exposing the settlement command path to consumers.
func NewSettlementEvidenceReader(database gdb.DB) (SettlementEvidenceReader, error) {
	if isNilFinancialSettlementDependency(database) {
		return nil, gerror.New("settlement evidence reader requires a database")
	}
	return &settlementEvidenceReader{database: database}, nil
}

type serviceImpl struct {
	database gdb.DB
	random   io.Reader
}

var _ BalanceService = (*serviceImpl)(nil)
var _ InvoiceService = (*serviceImpl)(nil)
var _ InvoiceTransformService = (*serviceImpl)(nil)
var _ FinancialAdjustmentService = (*serviceImpl)(nil)

// NewBalanceService constructs the internal balance and ledger service.
func NewBalanceService(database gdb.DB) (BalanceService, error) {
	return newBalanceService(database, rand.Reader)
}

// NewInvoiceService constructs the internal invoice lifecycle service.
func NewInvoiceService(database gdb.DB) (InvoiceService, error) {
	return newBalanceService(database, rand.Reader)
}

// NewInvoiceTransformService constructs the internal invoice merge/split service.
func NewInvoiceTransformService(database gdb.DB) (InvoiceTransformService, error) {
	return newBalanceService(database, rand.Reader)
}

// NewFinancialAdjustmentService constructs the reviewed financial adjustment service.
func NewFinancialAdjustmentService(database gdb.DB) (FinancialAdjustmentService, error) {
	return newBalanceService(database, rand.Reader)
}

// NewFinancialSettlementService constructs the transaction-local settlement
// coordinator with an explicitly injected outbox writer.
func NewFinancialSettlementService(
	database gdb.DB,
	events outbox.Enqueuer,
	credits CreditReservationService,
) (FinancialSettlementService, error) {
	return newFinancialSettlementService(database, events, credits)
}

func newBalanceService(database gdb.DB, random io.Reader) (*serviceImpl, error) {
	if database == nil {
		return nil, gerror.New("billing balance service requires a database")
	}
	if random == nil {
		return nil, gerror.New("billing balance service requires a random source")
	}
	return &serviceImpl{database: database, random: random}, nil
}
