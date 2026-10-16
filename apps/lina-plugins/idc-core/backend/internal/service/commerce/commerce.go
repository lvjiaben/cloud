// Package commerce owns generic cart intent, immutable quotes, and
// guest-to-customer ownership transitions.
package commerce

import (
	"context"
	"crypto/rand"
	"io"
	"regexp"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const maximumCartItems = 100

const (
	CartItemStatusActive       = "active"
	CartItemStatusNeedsReprice = "needs_reprice"
	CartItemStatusUnavailable  = "unavailable"
)

const (
	CartRepairRemove         = "remove"
	CartRepairReplaceVersion = "replace_version"
	CartRepairRequote        = "requote"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMMERCE_CART_INVALID",
		"The shopping cart request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMMERCE_CART_NOT_ACCESSIBLE",
		"The shopping cart is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeCurrencyConflict = bizerr.MustDefine(
		"IDC_COMMERCE_CART_CURRENCY_CONFLICT",
		"The shopping cart already uses a different currency",
		gcode.CodeInvalidOperation,
	)
	CodeCurrencySwitchBlocked = bizerr.MustDefine(
		"IDC_COMMERCE_CART_CURRENCY_SWITCH_BLOCKED",
		"Remove the shopping cart items before switching currency",
		gcode.CodeInvalidOperation,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMMERCE_CART_CONFLICT",
		"The shopping cart changed; reload it and try again",
		gcode.CodeInvalidOperation,
	)
	CodeCapacityExceeded = bizerr.MustDefine(
		"IDC_COMMERCE_CART_CAPACITY_EXCEEDED",
		"The shopping cart contains too many configuration items",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMMERCE_CART_UNAVAILABLE",
		"The shopping cart is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeContractPolicyInvalid = bizerr.MustDefine(
		"IDC_COMMERCE_CONTRACT_POLICY_INVALID",
		"The order contract rule is invalid; review the customer, product, and order settings",
		gcode.CodeInvalidOperation,
	)
	CodeContractTemplateUnavailable = bizerr.MustDefine(
		"IDC_COMMERCE_CONTRACT_TEMPLATE_UNAVAILABLE",
		"The selected contract template is unavailable; choose an enabled matching template",
		gcode.CodeInvalidOperation,
	)
	CodeContractTemplateConflict = bizerr.MustDefine(
		"IDC_COMMERCE_CONTRACT_TEMPLATE_CONFLICT",
		"The products require different contract templates; select one order template before payment",
		gcode.CodeInvalidOperation,
	)
	CodeOrderNotCancelable = bizerr.MustDefine(
		"IDC_COMMERCE_ORDER_NOT_CANCELABLE",
		"The order cannot be cancelled in its current state",
		gcode.CodeInvalidOperation,
	)
	CodeOrderCancellationPending = bizerr.MustDefine(
		"IDC_COMMERCE_ORDER_CANCELLATION_PENDING",
		"The order cancellation is waiting for payment closure",
		gcode.CodeInvalidOperation,
	)
	CodePaymentPreparationInvalid = bizerr.MustDefine(
		"IDC_COMMERCE_PAYMENT_PREPARATION_INVALID",
		"The payment preparation request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodePaymentPreparationNotAccessible = bizerr.MustDefine(
		"IDC_COMMERCE_PAYMENT_PREPARATION_NOT_ACCESSIBLE",
		"The payment preparation context is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodePaymentPreparationConflict = bizerr.MustDefine(
		"IDC_COMMERCE_PAYMENT_PREPARATION_CONFLICT",
		"The order payment context changed; reload it and try again",
		gcode.CodeInvalidOperation,
	)
	CodePaymentPreparationUnavailable = bizerr.MustDefine(
		"IDC_COMMERCE_PAYMENT_PREPARATION_UNAVAILABLE",
		"Payment preparation is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeOrderDirectoryInvalid = bizerr.MustDefine(
		"IDC_COMMERCE_ORDER_DIRECTORY_INVALID",
		"The order query is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeOrderDirectoryNotAccessible = bizerr.MustDefine(
		"IDC_COMMERCE_ORDER_DIRECTORY_NOT_ACCESSIBLE",
		"The order is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeOrderDirectoryUnavailable = bizerr.MustDefine(
		"IDC_COMMERCE_ORDER_DIRECTORY_UNAVAILABLE",
		"The order directory is temporarily unavailable",
		gcode.CodeInternalError,
	)
)

// AddItemInput accepts only public signed catalog facts and cart concurrency state.
type AddItemInput struct {
	ExpectedCartVersion int64
	Quantity            int
	Purchase            catalog.PurchaseConfigurationInput
}

// CartItem is one purchase intent and its current, read-only repair projection.
type CartItem struct {
	CartItemNo            string
	ProductNameZhCN       string
	ProductNameEnUS       string
	PublicSlug            string
	ConfigurationRequired bool
	ConfigurationFrozen   bool
	ConfigurationRevision string
	Quantity              int
	PurchaseCycles        int
	StatusCode            string
	IssueCode             string
	RepairAction          string
	Version               int64
}

// Cart is the bounded owner-scoped cart projection.
type Cart struct {
	CartNo       string
	CurrencyCode string
	Version      int64
	Items        []CartItem
	LatestQuote  *Quote
}

// GuestCartAccess returns the high-entropy ownership token only at creation.
type GuestCartAccess struct {
	Token string
	Cart  *Cart
}

// Service deliberately separates guest-token and authenticated-customer methods.
type Service interface {
	AddonService
	CreateGuestCart(context.Context, string) (*GuestCartAccess, error)
	GetGuestCart(context.Context, string) (*Cart, error)
	AddGuestItem(context.Context, string, AddItemInput) (*Cart, error)
	RemoveGuestItem(context.Context, string, string, int64, int64) (*Cart, error)
	SwitchGuestCurrency(context.Context, string, string, int64) (*Cart, error)
	OpenAccountCart(context.Context, string) (*Cart, error)
	GetAccountCart(context.Context) (*Cart, error)
	AddAccountItem(context.Context, AddItemInput) (*Cart, error)
	RemoveAccountItem(context.Context, string, int64, int64) (*Cart, error)
	SwitchAccountCurrency(context.Context, string, int64) (*Cart, error)
	MergeGuestCart(context.Context, string) (*Cart, error)
	QuoteGuestCart(context.Context, string, QuoteRequest) (*Quote, error)
	QuoteAccountCart(context.Context, QuoteRequest) (*Quote, error)
	CheckoutAccountQuote(context.Context, CheckoutRequest) (*Checkout, error)
	FreezeAccountQuoteItemConfiguration(context.Context, FreezeQuoteItemConfigurationInput) (*FrozenQuoteItemConfiguration, error)
	// CancelAccountOrder derives customer ownership from the active principal.
	// Payment-processing orders are not released until the payment owner proves closure.
	CancelAccountOrder(context.Context, OrderCancellationRequest) (*OrderCancellation, error)
}

type serviceImpl struct {
	currencies              CheckoutCurrencyReader
	database                gdb.DB
	catalog                 catalog.PurchaseConfigurationResolver
	quoteCatalog            catalog.PurchaseQuoteAssembler
	adjustments             QuoteAdjustmentEvaluator
	reservations            QuoteReservationFinalizer
	hasReservationFinalizer bool
	reservationReverser     QuoteReservationReverser
	hasReservationReverser  bool
	outbox                  checkoutEventWriter
	settlements             billing.FinancialSettlementService
	contractPolicy          contractpolicy.Resolver
	principal               principal.Reader
	random                  io.Reader
	structuredConfiguration *structuredConfigurationService
}

type checkoutEventWriter interface {
	Enqueue(context.Context, outbox.EnqueueInput) (*outbox.Event, error)
}

var _ Service = (*serviceImpl)(nil)

// New constructs the transaction-local cart service.
func New(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	principalReader principal.Reader,
	settlements billing.FinancialSettlementService,
) (Service, error) {
	return NewWithContractTemplates(
		database, catalogResolver, principalReader, settlements, nil,
	)
}

// NewWithContractTemplates enables fail-closed order contract resolution when
// the compliance plugin supplies its installation-local template directory.
func NewWithContractTemplates(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	principalReader principal.Reader,
	settlements billing.FinancialSettlementService,
	templates contracttemplatecap.Source,
) (Service, error) {
	marketingService, err := marketing.New(database, marketing.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create commerce marketing service")
	}
	evaluator, err := NewMarketingQuoteAdjustmentEvaluator(marketingService)
	if err != nil {
		return nil, err
	}
	service, err := newServiceWithQuoteAdjustments(
		database, catalogResolver, principalReader, evaluator, settlements,
	)
	if err != nil {
		return nil, err
	}
	service.contractPolicy = contractpolicy.NewResolver(templates)
	return service, nil
}

// NewWithQuoteAdjustments installs a transaction-aware quote-wide adjustment
// evaluator. The default constructor installs the local marketing owner;
// callers use this variant only for an explicit alternative policy source.
func NewWithQuoteAdjustments(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	principalReader principal.Reader,
	evaluator QuoteAdjustmentEvaluator,
	settlements billing.FinancialSettlementService,
) (Service, error) {
	return newServiceWithQuoteAdjustments(
		database, catalogResolver, principalReader, evaluator, settlements,
	)
}

func newServiceWithQuoteAdjustments(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	principalReader principal.Reader,
	evaluator QuoteAdjustmentEvaluator,
	settlements billing.FinancialSettlementService,
) (*serviceImpl, error) {
	service, err := newService(
		database, catalogResolver, principalReader, rand.Reader, settlements,
	)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, gerror.New("commerce service requires a quote adjustment evaluator")
	}
	service.adjustments = evaluator
	if finalizer, ok := evaluator.(QuoteReservationFinalizer); ok {
		service.reservations = finalizer
		service.hasReservationFinalizer = true
	}
	if reverser, ok := evaluator.(QuoteReservationReverser); ok {
		service.reservationReverser = reverser
		service.hasReservationReverser = true
	}
	return service, nil
}

func newService(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	principalReader principal.Reader,
	random io.Reader,
	settlements billing.FinancialSettlementService,
) (*serviceImpl, error) {
	if database == nil {
		return nil, gerror.New("commerce service requires a database")
	}
	if catalogResolver == nil {
		return nil, gerror.New("commerce service requires a catalog resolver")
	}
	if principalReader == nil {
		return nil, gerror.New("commerce service requires a principal reader")
	}
	if random == nil {
		return nil, gerror.New("commerce service requires a random source")
	}
	if settlements == nil {
		return nil, gerror.New("commerce service requires a financial settlement service")
	}
	outboxService, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create commerce outbox service")
	}
	quoteCatalog, _ := catalogResolver.(catalog.PurchaseQuoteAssembler)
	return &serviceImpl{
		database: database, catalog: catalogResolver, quoteCatalog: quoteCatalog,
		adjustments: noQuoteAdjustments{}, reservations: noQuoteReservationFinalizer{},
		reservationReverser: noQuoteReservationReverser{},
		outbox:              outboxService, settlements: settlements,
		contractPolicy: contractpolicy.NewResolver(nil),
		principal:      principalReader, random: random,
	}, nil
}

// NewWithQuoteAdjustmentsAndContractTemplates reuses the shared pricing policy
// and preserves the same contract gate for channel and website checkout.
func NewWithQuoteAdjustmentsAndContractTemplates(database gdb.DB, resolver catalog.PurchaseConfigurationResolver, reader principal.Reader, evaluator QuoteAdjustmentEvaluator, settlements billing.FinancialSettlementService, templates contracttemplatecap.Source) (Service, error) {
	service, err := newServiceWithQuoteAdjustments(database, resolver, reader, evaluator, settlements)
	if err != nil {
		return nil, err
	}
	service.contractPolicy = contractpolicy.NewResolver(templates)
	return service, nil
}
