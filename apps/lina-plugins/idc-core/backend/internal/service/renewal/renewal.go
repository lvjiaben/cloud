// Package renewal owns immutable entitlement renewal quotes, invoices, automatic
// balance attempts, and the core-side business-operation convergence.
package renewal

import (
	"context"
	"crypto/rand"
	"io"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	renewalownerspi "lina-plugin-idc-core/backend/cap/renewalownercap/spi"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/triallifecycle"
)

const (
	RequestSourceCustomer = "customer"
	RequestSourceSystem   = "system"

	defaultQuoteLifetime   = 15 * time.Minute
	automaticQuoteLifetime = 24 * time.Hour
	maximumQuoteAssets     = 64
	maximumServiceOptions  = 5_000
	maximumAdjustments     = 4_096
)

var (
	businessReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	canonicalDigestPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)

	CodeInvalid = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_INVALID", "The entitlement renewal request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_NOT_ACCESSIBLE", "The entitlement renewal is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_CONFLICT", "The entitlement renewal conflicts with current facts",
		gcode.CodeInvalidOperation,
	)
	CodeCapabilityUnavailable = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_CAPABILITY_UNAVAILABLE",
		"The entitlement renewal capability is unavailable or requires manual review",
		gcode.CodeInvalidOperation,
	)
	CodePriceUnavailable = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_PRICE_UNAVAILABLE",
		"The current entitlement has no valid renewal price",
		gcode.CodeInvalidOperation,
	)
	CodeBalanceInsufficient = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_BALANCE_INSUFFICIENT",
		"The same-currency customer balance is insufficient for this renewal",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_ENTITLEMENT_RENEWAL_UNAVAILABLE", "The entitlement renewal is temporarily unavailable",
		gcode.CodeInternalError,
	)
)

// QuoteInput requests a current-price renewal quote for one owned service.
// The raw idempotency key is hashed in memory and is never persisted.
type QuoteInput struct {
	// FormalBillingRuleNo selects a current recurring period for a trial. Empty means ordinary renewal.
	FormalBillingRuleNo string
	EntitlementNo       string
	RenewalCycles       int
	IdempotencyKey      string
	AssetNumbers        []string
}

// Quote is the bounded customer-visible immutable renewal quote projection.
type Quote struct {
	RequestNo          string
	QuoteNo            string
	PriceSnapshotNo    string
	EntitlementNo      string
	CurrencyCode       string
	RenewalCycles      int
	RenewalMode        string
	OriginalExpiresAt  time.Time
	ProjectedExpiresAt time.Time
	SubtotalMinor      int64
	DiscountMinor      int64
	NetAmountMinor     int64
	TaxAmountMinor     int64
	TotalAmountMinor   int64
	IssuedAt           time.Time
	ExpiresAt          time.Time
	Reused             bool
}

// ConfirmInput confirms one still-current renewal quote as a payable business intent.
type ConfirmInput struct {
	QuoteNo        string
	IdempotencyKey string
}

// PayInput settles the invoice derived from one owned renewal operation. The
// customer, invoice, currency, amount, and ledger source are never caller-set.
type PayInput struct {
	OperationNo string
}

// PolicyInput appends a customer-owned automatic renewal policy revision.
// ExpectedEntitlementVersion prevents a stale page from replacing newer service
// or policy facts.
type PolicyInput struct {
	EntitlementNo              string
	ExpectedEntitlementVersion int64
	AutoRenewEnabled           bool
	RenewalCycles              int
	LeadSeconds                int64
	MaximumAttempts            int
	RetryIntervalSeconds       int64
	FailureNotificationEnabled bool
}

// Operation is the bounded renewal invoice and business-operation projection.
type Operation struct {
	OperationNo      string
	QuoteNo          string
	InvoiceNo        string
	EntitlementNo    string
	CurrencyCode     string
	Status           string
	RenewalMode      string
	TotalAmountMinor int64
	PaymentDueAt     time.Time
	Reused           bool
}

// Settlement is the bounded result of an atomic invoice balance payment and
// the replay-safe business-operation convergence that follows it.
type Settlement struct {
	balancePayment      *billing.InvoiceBalancePayment
	Operation           *Operation
	LedgerTransactionNo string
	CreditExposureNo    string
	Reused              bool
}

// AutomaticAttempt is one append-only scheduler outcome. Expected balance
// shortages are returned as data so the job can notify without treating the
// retry policy as an infrastructure failure.
type AutomaticAttempt struct {
	AttemptNo           string
	OperationNo         string
	InvoiceNo           string
	EntitlementNo       string
	CustomerNo          string
	CurrencyCode        string
	PolicyNo            string
	PolicyVersion       int64
	Sequence            int
	Outcome             string
	LedgerTransactionNo string
	FailureCode         string
	NextAttemptAt       *time.Time
	Reused              bool
}

// AutomaticBatch reports one bounded due-service scan without exposing
// customer balances or Provider identifiers.
type AutomaticBatch struct {
	Selected            int
	Processed           int
	InsufficientBalance int
	Settled             int
	Exhausted           int
	Failures            int
}

// Policy is the current immutable automatic renewal policy projection.
type Policy struct {
	PolicyNo                   string
	EntitlementNo              string
	PolicyVersion              int64
	EntitlementVersion         int64
	AutoRenewEnabled           bool
	RenewalCycles              int
	LeadSeconds                int64
	MaximumAttempts            int
	RetryIntervalSeconds       int64
	FailureNotificationEnabled bool
	Reused                     bool
}

// Service exposes only customer-owned renewal commands. System automatic
// renewal uses a separate trusted scheduler boundary added below this interface.
type Service interface {
	// ListTrialConversions returns one owned, bounded trial page and formal period choices.
	ListTrialConversions(context.Context, int, int) (*TrialConversionPage, error)
	QuoteCurrentCustomer(context.Context, QuoteInput) (*Quote, error)
	ConfirmCurrentCustomer(context.Context, ConfirmInput) (*Operation, error)
	PayCurrentCustomer(context.Context, PayInput) (*Settlement, error)
	ConfigurePolicyCurrentCustomer(context.Context, PolicyInput) (*Policy, error)
}

// ManagementService is the trusted staff renewal boundary. The caller must
// complete exact staff authorization before using it; customer ownership is
// still supplied explicitly and rechecked by the renewal transaction.
type ManagementService interface {
	QuoteForStaff(context.Context, string, QuoteInput) (*Quote, error)
	ConfirmForStaff(context.Context, string, ConfirmInput, int) (*Operation, error)
}

// ChannelService is the trusted downstream-channel boundary. The service
// owner remains the renewal subject while the separately supplied, verified
// enterprise payer owns the invoice, balance debit, or credit exposure.
type ChannelService interface {
	QuoteForChannel(context.Context, string, QuoteInput) (*Quote, error)
	ConfirmForChannel(context.Context, string, string, ConfirmInput, string) (*Operation, error)
	PayForChannel(context.Context, string, string, PayInput, string, string) (*Settlement, error)
}

// AutomaticService is a trusted scheduler boundary. It accepts no customer,
// currency, amount, invoice, or Provider identifiers from a public caller.
type AutomaticService interface {
	AttemptDueService(context.Context, string) (*AutomaticAttempt, error)
	RunAutomaticRenewals(context.Context, int) (*AutomaticBatch, error)
}

// ProviderService is the trusted worker boundary that submits and observes
// resource-owner tasks. Public callers never supply task, Provider, or route facts.
type ProviderService interface {
	RunProviderOperations(context.Context, int) (*ProviderBatch, error)
	DispatchProviderOperation(context.Context, string) (*Operation, error)
	ReconcileProviderOperation(context.Context, string) (*Operation, error)
}

// RuntimeService is the complete local renewal runtime used by HTTP and jobs.
type RuntimeService interface {
	billing.RenewalInvoicePayment
	Service
	ManagementService
	ChannelService
	AutomaticService
	ProviderService
}

type serviceImpl struct {
	database     gdb.DB
	principal    principal.Reader
	catalog      catalog.RenewalQuoteAssembler
	priceChoices catalog.ConfigurationPriceSelectionReader
	adjustments  commerce.QuoteAdjustmentEvaluator
	reservations commerce.QuoteReservationFinalizer
	invoices     billing.InvoiceService
	owners       renewalownercap.Router
	recovery     triallifecycle.RecoveryScheduler
	random       io.Reader
}

var _ Service = (*serviceImpl)(nil)
var _ ManagementService = (*serviceImpl)(nil)
var _ ChannelService = (*serviceImpl)(nil)
var _ AutomaticService = (*serviceImpl)(nil)
var _ ProviderService = (*serviceImpl)(nil)

// New constructs the production renewal service from local core-owned dependencies.
func New(ctx context.Context, database gdb.DB, principalReader principal.Reader, catalogReader catalog.CatalogReader) (RuntimeService, error) {
	evaluator, err := newDefaultQuoteAdjustments(database)
	if err != nil {
		return nil, err
	}
	return NewWithQuoteAdjustments(ctx, database, principalReader, catalogReader, evaluator)
}

// NewWithQuoteAdjustments injects the same customer pricing policy used by checkout.
func NewWithQuoteAdjustments(ctx context.Context, database gdb.DB, principalReader principal.Reader, catalogReader catalog.CatalogReader, evaluator commerce.QuoteAdjustmentEvaluator) (RuntimeService, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || principalReader == nil || catalogReader == nil || evaluator == nil {
		return nil, gerror.New("renewal service requires database, principal, catalog and pricing")
	}
	owners, err := renewalownerspi.Freeze().Build(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "construct renewal owner router")
	}
	service, err := newRenewalWorkerService(database, catalogReader, owners, evaluator)
	if err != nil {
		return nil, err
	}
	service.principal = principalReader
	return service, nil
}

// NewWorkers constructs the trusted automatic and Provider renewal workers.
// The returned service has no customer-principal dependency and is not exposed
// through HTTP routes.
func NewWorkers(ctx context.Context, database gdb.DB, catalogReader catalog.CatalogReader, owners renewalownercap.Router) (AutomaticService, ProviderService, error) {
	evaluator, err := newDefaultQuoteAdjustments(database)
	if err != nil {
		return nil, nil, err
	}
	return NewWorkersWithQuoteAdjustments(ctx, database, catalogReader, owners, evaluator)
}

// NewWorkersWithQuoteAdjustments gives automatic renewals the customer pricing policy.
func NewWorkersWithQuoteAdjustments(ctx context.Context, database gdb.DB, catalogReader catalog.CatalogReader, owners renewalownercap.Router, evaluator commerce.QuoteAdjustmentEvaluator) (AutomaticService, ProviderService, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || catalogReader == nil || owners == nil || evaluator == nil {
		return nil, nil, gerror.New("renewal workers require database, catalog, pricing and resource source")
	}
	service, err := newRenewalWorkerService(database, catalogReader, owners, evaluator)
	if err != nil {
		return nil, nil, err
	}
	return service, service, nil
}

func newDefaultQuoteAdjustments(database gdb.DB) (commerce.QuoteAdjustmentEvaluator, error) {
	marketingService, err := marketing.New(database, marketing.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create renewal marketing service")
	}
	return commerce.NewMarketingQuoteAdjustmentEvaluator(marketingService)
}

func newRenewalWorkerService(
	database gdb.DB,
	catalogReader catalog.CatalogReader,
	owners renewalownercap.Router,
	evaluator commerce.QuoteAdjustmentEvaluator,
) (*serviceImpl, error) {
	assembler, err := catalog.RenewalQuoteAssemblerFromPublicReader(catalogReader)
	if err != nil {
		return nil, err
	}
	priceChoices, err := catalog.ConfigurationPriceSelectionReaderFromPublicReader(catalogReader)
	if err != nil {
		return nil, err
	}
	reservations, ok := evaluator.(commerce.QuoteReservationFinalizer)
	if !ok {
		return nil, gerror.New("renewal marketing service does not finalize reservations")
	}
	invoiceService, err := billing.NewInvoiceService(database)
	if err != nil {
		return nil, err
	}
	return &serviceImpl{
		database: database, catalog: assembler, priceChoices: priceChoices, adjustments: evaluator,
		reservations: reservations, invoices: invoiceService,
		owners: owners, random: rand.Reader,
	}, nil
}

// TrialConversionPeriod is one current formal billing schedule, never a client price.
type TrialConversionPeriod struct {
	BillingRuleNo  string
	PeriodUnitCode string
	PeriodCount    int
}

// TrialConversionItem presents a current customer's local trial and safe choices.
type TrialConversionItem struct {
	EntitlementNo, ProductNameZhCN, ProductNameEnUS, CurrencyCode, StatusCode string
	ExpiresAt                                                                 *time.Time
	Periods                                                                   []TrialConversionPeriod
}

// TrialConversionPage includes the exact owned count and a page of at most 100.
type TrialConversionPage struct {
	Items []TrialConversionItem
	Total int
}
