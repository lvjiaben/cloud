// Package taxinvoiceredletter owns supplier-neutral refund allocation and
// red-letter correction state. It never delays or mutates the core refund.
package taxinvoiceredletter

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const (
	PermissionQuery  = "idc:admin:compliance:red-letter:query"
	PermissionHandle = "idc:admin:compliance:red-letter:handle"

	StatusAwaitingConfirmation = "awaiting_confirmation"
	StatusCompleted            = "completed"
	StatusManualRequired       = "manual_required"

	ManualReasonAllocationSelection = "allocation_selection_required"
	ManualReasonProviderUnavailable = "provider_automation_unavailable"
)

// Task is the safe workflow projection. Private evidence locations and staff
// identity details are deliberately absent.
type Task struct {
	TaskNo                       string
	RefundOperationNo            string
	TaxInvoiceNo                 string
	CustomerNo                   string
	CurrencyCode                 string
	StatusCode                   string
	ManualReasonCode             string
	AllocationResolved           bool
	CustomerConfirmationRequired bool
	GrossAmountMinor             int64
	NetAmountMinor               int64
	TaxAmountMinor               int64
	Version                      int64
	CreatedAt                    time.Time
	CompletedAt                  *time.Time
	CandidateCount               int
	AllocationCount              int
	CustomerActionURL            string
	CompletionEvidenceNo         string
}

type AdminListQuery struct {
	Page                           int
	Cursor, CustomerNo, StatusCode string
	Limit                          int
}
type AdminPage struct {
	Total      int
	Items      []Task
	NextCursor string
}

type Candidate struct {
	RefundAllocationNo, TaxInvoiceNo, TaxInvoiceAllocationNo, TaxInvoiceItemNo     string
	RequestedGrossMinor, AvailableGrossMinor, AvailableNetMinor, AvailableTaxMinor int64
}

type Allocation struct {
	RefundAllocationNo, TaxInvoiceNo, TaxInvoiceAllocationNo, TaxInvoiceItemNo string
	GrossAmountMinor, NetAmountMinor, TaxAmountMinor                           int64
}

type Evidence struct {
	EvidenceNo, EvidenceReference, EvidenceSHA256, MediaType string
	ByteSize                                                 int64
	OccurredAt                                               time.Time
}

type AdminDetail struct {
	Task             Task
	Candidates       []Candidate
	Allocations      []Allocation
	Evidence         *Evidence
	HistoryTruncated bool
}

// RecordResult reports the created/replayed task, or that the refund had no
// already-issued tax amount requiring correction.
type RecordResult struct {
	Task                 *Task
	Reused               bool
	NoCorrectionRequired bool
}

// AllocationSelection is one explicit eligible original-invoice choice.
type AllocationSelection struct {
	RefundAllocationNo     string
	TaxInvoiceAllocationNo string
	GrossAmountMinor       int64
}

// ResolveInput resolves an ambiguous refund without guessing invoice order.
type ResolveInput struct {
	TaskNo          string
	ExpectedVersion int64
	IdempotencyKey  string
	Selections      []AllocationSelection
}

// ManualCompletionInput records real red-letter evidence. UploadedObjectRef
// is an opaque private-storage reference, never returned by this service.
type ManualCompletionInput struct {
	TaskNo            string
	ExpectedVersion   int64
	IdempotencyKey    string
	EvidenceReference string
	EvidenceSHA256    string
	UploadedObjectRef string
	MediaType         string
	ByteSize          int64
	OccurredAt        time.Time
}

// RecheckInput asks the configured adapter for an authoritative customer
// confirmation result. The customer's click is never itself confirmation.
type RecheckInput struct {
	TaskNo          string
	ExpectedVersion int64
	IdempotencyKey  string
}

// AuthoritativeResult is the bounded provider-neutral recheck outcome.
type AuthoritativeResult struct {
	Confirmed         bool
	Completed         bool
	ResultUnknown     bool
	CustomerActionURL string
	EvidenceReference string
	EvidenceSHA256    string
	OccurredAt        time.Time
}

// Gateway is implemented by a verified supplier adapter. The closed default
// performs no network call and cannot manufacture confirmation.
type Gateway interface {
	// CustomerAction returns a short supplier action URL for an already-created task.
	CustomerAction(context.Context, string, string) (string, error)
	// Recheck returns only the supplier's authoritative state and immutable evidence metadata.
	Recheck(context.Context, string, string) (AuthoritativeResult, error)
}

// ClosedGateway keeps NuoNuo red-letter calls unavailable until task 10.11.e.
type ClosedGateway struct{}

// CustomerAction fails closed while the verified supplier adapter is unavailable.
func (ClosedGateway) CustomerAction(context.Context, string, string) (string, error) {
	return "", gerror.New("red-letter provider automation is unavailable")
}

// Recheck fails closed while the verified supplier adapter is unavailable.
func (ClosedGateway) Recheck(context.Context, string, string) (AuthoritativeResult, error) {
	return AuthoritativeResult{}, gerror.New("red-letter provider automation is unavailable")
}

// Service exposes system ingestion, separately authorized staff actions, and
// ownership-only customer reads/rechecks.
type Service interface {
	// RecordPostedRefund consumes one core-authoritative posted refund idempotently.
	RecordPostedRefund(context.Context, string, string) (*RecordResult, error)
	// GetAdmin returns one task after enforcing the exact query permission.
	GetAdmin(context.Context, string) (*Task, error)
	ListAdmin(context.Context, AdminListQuery) (*AdminPage, error)
	GetAdminDetail(context.Context, string) (*AdminDetail, error)
	// ResolveAllocation stores a complete staff-selected plan for an ambiguous refund.
	ResolveAllocation(context.Context, ResolveInput) (*Task, error)
	// CompleteManual closes one resolved task only with immutable real-world evidence.
	CompleteManual(context.Context, ManualCompletionInput) (*Task, error)
	// GetCustomer returns only the current customer's own safe task projection.
	GetCustomer(context.Context, string) (*Task, error)
	// RecheckCustomer asks the supplier for authoritative state; the click itself proves nothing.
	RecheckCustomer(context.Context, RecheckInput) (*Task, error)
}

type serviceImpl struct {
	database      gdb.DB
	source        taxinvoicecap.Service
	customers     identitycap.CurrentCustomerReader
	management    managementcap.Runtime
	customerAudit idccap.BusinessAuditWriter
	systemAudit   idccap.BusinessAuditWriter
	gateway       Gateway
	now           func() time.Time
}

var _ Service = (*serviceImpl)(nil)

// New constructs one startup-owned red-letter service graph.
func New(
	database gdb.DB,
	source taxinvoicecap.Service,
	customers identitycap.CurrentCustomerReader,
	management managementcap.Runtime,
	customerAudit idccap.BusinessAuditWriter,
	systemAudit idccap.BusinessAuditWriter,
	gateway Gateway,
) (Service, error) {
	if nilDependency(database) || nilDependency(source) || nilDependency(customers) ||
		managementcap.ValidateRuntime(management) != nil || nilDependency(customerAudit) ||
		nilDependency(systemAudit) || nilDependency(gateway) {
		return nil, gerror.New("tax invoice red-letter dependencies are invalid")
	}
	return &serviceImpl{
		database: database, source: source, customers: customers, management: management,
		customerAudit: customerAudit, systemAudit: systemAudit, gateway: gateway, now: time.Now,
	}, nil
}

// nilDependency detects absent interfaces, including typed nil values.
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
