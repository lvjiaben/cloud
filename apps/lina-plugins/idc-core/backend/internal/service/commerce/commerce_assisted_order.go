// This file defines the staff-assisted quote contract. Manual pricing is
// represented only as a fixed discount over an authoritative fresh quote.

package commerce

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

const (
	maximumAssistedQuoteItems     = 8
	defaultAssistedQuotePageLimit = 25
	maximumAssistedQuotePageLimit = 100
)

// AssistedPaymentTerm is a prepayment-only staff-selected payment deadline policy.
type AssistedPaymentTerm string

const (
	AssistedPaymentTermStandardPrepay AssistedPaymentTerm = "standard_prepay"
	AssistedPaymentTermExtendedPrepay AssistedPaymentTerm = "extended_prepay"
)

// AssistedQuoteStatus is the append-only request workflow state.
type AssistedQuoteStatus string

const (
	assistedQuoteStatusDirectReady     AssistedQuoteStatus = "direct_ready"
	AssistedQuoteStatusPendingApproval AssistedQuoteStatus = "pending_approval"
	AssistedQuoteStatusApproved        AssistedQuoteStatus = "approved"
	AssistedQuoteStatusRejected        AssistedQuoteStatus = "rejected"
	AssistedQuoteStatusOrdered         AssistedQuoteStatus = "ordered"
)

// AssistedQuoteDecision is an independent staff decision.
type AssistedQuoteDecision string

const (
	AssistedQuoteDecisionApproved AssistedQuoteDecision = "approved"
	AssistedQuoteDecisionRejected AssistedQuoteDecision = "rejected"
)

var (
	CodeAssistedQuoteInvalid = bizerr.MustDefine(
		"IDC_COMMERCE_ASSISTED_QUOTE_INVALID",
		"The assisted quote request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeAssistedQuoteNotAccessible = bizerr.MustDefine(
		"IDC_COMMERCE_ASSISTED_QUOTE_NOT_ACCESSIBLE",
		"The assisted quote request is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeAssistedQuoteConflict = bizerr.MustDefine(
		"IDC_COMMERCE_ASSISTED_QUOTE_CONFLICT",
		"The assisted quote changed; reload it and try again",
		gcode.CodeInvalidOperation,
	)
	CodeAssistedQuoteUnavailable = bizerr.MustDefine(
		"IDC_COMMERCE_ASSISTED_QUOTE_UNAVAILABLE",
		"The assisted quote service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)

// AssistedQuoteItemInput contains one public signed product configuration.
type AssistedQuoteItemInput struct {
	Quantity int
	Purchase catalog.PurchaseConfigurationInput
}

// AssistedQuoteCreateInput records bounded staff intent, never a paid or service state.
type AssistedQuoteCreateInput struct {
	CustomerNo          string
	CurrencyCode        string
	ManualDiscountMinor int64
	PaymentDueAt        int64
	Reason              string
	InternalNote        string
	IdempotencyKey      string

	Items []AssistedQuoteItemInput
}

// AssistedQuoteFilter is a bounded, encrypted-cursor management query.
type AssistedQuoteFilter struct {
	Page       int
	CustomerNo string
	StatusCode AssistedQuoteStatus
	Cursor     string
	Limit      int
}

// AssistedQuoteItem is one display-safe request line.
type AssistedQuoteItem struct {
	RequestItemNo   string
	LineNumber      int
	ProductNameZhCN string
	ProductNameEnUS string
	Quantity        int
	PurchaseCycles  int
}

// AssistedQuoteRequest is the bounded management projection.
type AssistedQuoteRequest struct {
	RequestNo            string
	CustomerNo           string
	CustomerCode         string
	CustomerDisplayName  string
	CustomerPrimaryPhone string
	CurrencyCode         string
	StatusCode           AssistedQuoteStatus
	ManualDiscountMinor  int64
	PaymentTermCode      AssistedPaymentTerm
	PaymentDueSeconds    int

	Reason           string
	InternalNote     string
	RequestedBy      int
	DecisionCode     AssistedQuoteDecision
	DecisionBy       int
	DecisionReason   string
	QuoteNo          string
	OrderNo          string
	OrderStatusCode  string
	SubtotalMinor    int64
	DiscountMinor    int64
	TotalAmountMinor int64
	ExpiresAt        time.Time
	CreatedAt        time.Time
	DecisionAt       *time.Time
	Items            []AssistedQuoteItem
	CanApprove       bool
	CanReject        bool
	CanCheckout      bool
	Reused           bool
}

// AssistedQuotePage contains a bounded page and the same-scope filtered total.
type AssistedQuotePage struct {
	Total      int
	Requests   []AssistedQuoteRequest
	NextCursor string
}

// AssistedQuoteAuthorizer resolves trusted staff identity and exact permissions.
type AssistedQuoteAuthorizer interface {
	AuthorizeAssistedQuoteCreate(context.Context, string) (int, error)
	AuthorizeAssistedQuoteList(context.Context) (int, error)
	AuthorizeAssistedQuoteDetail(context.Context, string) (int, error)
}

// AssistedQuoteService owns direct staff assistance, immutable pricing and safe checkout.
type AssistedQuoteService interface {
	// CreateAssistedQuote atomically creates an order for an active customer; exact retries return the existing order. It never debits customer funds.
	CreateAssistedQuote(context.Context, AssistedQuoteCreateInput) (*AssistedQuoteRequest, error)
	// AssistedOrderCurrencies returns the configured merchant currency without relying on published product rows.
	AssistedOrderCurrencies(context.Context, string) ([]string, error)
	ListAssistedQuotes(context.Context, AssistedQuoteFilter) (*AssistedQuotePage, error)
	GetAssistedQuote(context.Context, string) (*AssistedQuoteRequest, error)
}

type assistedQuoteServiceImpl struct {
	currencies AssistedCurrencyReader
	database   gdb.DB
	catalog    catalog.PurchaseConfigurationResolver
	assembler  catalog.PurchaseQuoteAssembler
	commerce   *serviceImpl
	audit      idccap.BusinessAuditWriter
	authorizer AssistedQuoteAuthorizer
	cursor     *orderCursorCodec
	random     io.Reader
}

var _ AssistedQuoteService = (*assistedQuoteServiceImpl)(nil)

// NewAssistedQuoteService reuses the startup-owned commerce graph and local catalog.
func NewAssistedQuoteService(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	commerceService Service,
	auditWriter idccap.BusinessAuditWriter,
	authorizer AssistedQuoteAuthorizer,
	cursorKey []byte,
) (AssistedQuoteService, error) {
	commerceImpl, ok := commerceService.(*serviceImpl)
	assembler, hasAssembler := catalogResolver.(catalog.PurchaseQuoteAssembler)
	if isNilOrderDirectoryDependency(database) || isNilOrderDirectoryDependency(catalogResolver) ||
		!ok || commerceImpl == nil || !hasAssembler || assembler == nil ||
		isNilOrderDirectoryDependency(auditWriter) || isNilOrderDirectoryDependency(authorizer) {
		return nil, gerror.New("assisted quote service requires database, catalog, commerce, audit, and authorization services")
	}
	derivedCursorKey := sha256.Sum256(append(
		[]byte("idc-core/assisted-quote-cursor/v1\x00"), cursorKey...,
	))
	codec, err := newOrderCursorCodec(derivedCursorKey[:])
	clear(derivedCursorKey[:])
	if err != nil {
		return nil, err
	}
	return &assistedQuoteServiceImpl{
		database: database, catalog: catalogResolver, assembler: assembler,
		commerce: commerceImpl, audit: auditWriter, authorizer: authorizer,
		cursor: codec, random: rand.Reader,
	}, nil
}
