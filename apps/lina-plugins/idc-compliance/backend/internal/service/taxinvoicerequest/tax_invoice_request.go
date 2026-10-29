// Package taxinvoicerequest owns immutable, supplier-neutral customer tax-invoice requests.
// It seals buyer and seller snapshots, persists bounded billing allocations,
// and reports reserved usage back to the core billing owner without calling a provider.
package taxinvoicerequest

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const (
	// InvoiceTypePendingConfirmation keeps the customer request supplier-neutral until review.
	InvoiceTypePendingConfirmation invoiceTypeCode = "pending_confirmation"
	// StatusPendingReview is the only status created by the customer submission path.
	StatusPendingReview statusCode = "pending_review"
	// StatusRejected releases capacity only after an explicit pre-issuance review decision.
	StatusRejected statusCode = "rejected"
	// StatusCanceled releases capacity only after an explicit pre-issuance cancellation.
	StatusCanceled statusCode = "canceled"
)

type invoiceTypeCode string
type statusCode string

// CreateInput contains only customer-selectable fields. Tax facts, seller
// identity, status, and invoice type are intentionally absent.
type CreateInput struct {
	ProfileNo      string
	Selections     []taxinvoicecap.Selection
	IdempotencyKey string
}

// Request is the safe local result of one customer submission.
type Request struct {
	TaxInvoiceNo     string
	CustomerNo       string
	StatusCode       string
	InvoiceTypeCode  string
	CurrencyCode     string
	GrossAmountMinor int64
	NetAmountMinor   int64
	TaxAmountMinor   int64
	SubmittedAt      time.Time
	Version          int64
	Reused           bool
}

// CurrentCustomerQuery controls one bounded customer-owned request page.
type CurrentCustomerQuery struct {
	Cursor string
	Limit  int
}

// CurrentCustomerRequest is a customer-safe request history projection.
type CurrentCustomerRequest struct {
	TaxInvoiceNo       string
	StatusCode         string
	InvoiceTypeCode    string
	CurrencyCode       string
	GrossAmountMinor   int64
	MaskedInvoiceNo    string
	DecisionReasonCode string
	SubmittedAt        time.Time
	IssuedAt           *time.Time
	DeliveredAt        *time.Time
	Version            int64
}

// CurrentCustomerPage is one bounded page without an exact total count.
type CurrentCustomerPage struct {
	Items      []CurrentCustomerRequest
	NextCursor string
}

// Service owns request creation and implements the compliance allocation
// reader consumed by the core-owned source calculator.
type Service interface {
	// ListCurrentCustomerOptions returns bounded, current-customer picker data with reserved usage applied.
	ListCurrentCustomerOptions(context.Context) (*taxinvoicecap.RequestOptions, error)
	// CreateCurrentCustomer creates or exactly replays one immutable pending-review request.
	CreateCurrentCustomer(context.Context, CreateInput) (*Request, error)
	// ListCurrentCustomerRequests returns only the authenticated customer's safe history.
	ListCurrentCustomerRequests(context.Context, CurrentCustomerQuery) (*CurrentCustomerPage, error)
	// ReadAllocatedUsage returns reservation totals for only the supplied current customer and bounded invoice set.
	ReadAllocatedUsage(context.Context, string, []string) ([]taxinvoicecap.InvoiceUsage, error)
}

type serviceImpl struct {
	database    gdb.DB
	source      taxinvoicecap.Service
	accessor    secretcap.Accessor
	manager     secretcap.Manager
	customers   identitycap.CurrentCustomerReader
	privacyGate privacyCollectionGate
}

type privacyCollectionGate interface {
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
}

var (
	_ Service                             = (*serviceImpl)(nil)
	_ taxinvoicecap.AllocationUsageReader = (*serviceImpl)(nil)
)

// New constructs the local request service from the shared database, the
// core-owned source boundary, and one compliance-owned encrypted snapshot scope.
func New(
	database gdb.DB,
	source taxinvoicecap.Service,
	accessor secretcap.Accessor,
	manager secretcap.Manager,
	customers identitycap.CurrentCustomerReader,
) (Service, error) {
	return NewWithCollectionGate(database, source, accessor, manager, customers, openPrivacyCollectionGate{})
}

// NewWithCollectionGate blocks new invoice evidence collection until the
// merchant configures the matching retention policy.
func NewWithCollectionGate(
	database gdb.DB,
	source taxinvoicecap.Service,
	accessor secretcap.Accessor,
	manager secretcap.Manager,
	customers identitycap.CurrentCustomerReader,
	privacyGate privacyCollectionGate,
) (Service, error) {
	if nilDependency(database) || nilDependency(source) ||
		nilDependency(accessor) || nilDependency(manager) || nilDependency(customers) || nilDependency(privacyGate) {
		return nil, gerror.New("tax invoice request dependencies are invalid")
	}
	return &serviceImpl{
		database: database, source: source, accessor: accessor, manager: manager,
		customers: customers, privacyGate: privacyGate,
	}, nil
}

type openPrivacyCollectionGate struct{}

func (openPrivacyCollectionGate) RequireSensitiveCollection(context.Context, string, string) (time.Time, error) {
	return time.Now().UTC().Add(24 * time.Hour), nil
}

// ListCurrentCustomerOptions delegates source truth while applying this
// compliance owner's current reservation projection.
func (s *serviceImpl) ListCurrentCustomerOptions(
	ctx context.Context,
) (*taxinvoicecap.RequestOptions, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("tax invoice request options context is invalid")
	}
	options, err := s.source.ListCurrentCustomerOptions(ctx, s)
	if err != nil {
		return nil, gerror.Wrap(err, "list current customer tax invoice options")
	}
	if options == nil || !validBusinessNo(options.CustomerNo, 64) {
		return nil, gerror.New("current customer tax invoice options are unavailable")
	}
	return options, nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
