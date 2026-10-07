// Package taxinvoicemanagement owns staff review, local submission queuing,
// and safe pre-issuance cancellation for customer tax-invoice requests.
package taxinvoicemanagement

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// PermissionQuery allows staff to read safe tax-invoice request projections.
	PermissionQuery = "idc:admin:compliance:tax-invoice:query"
	// PermissionReview allows staff to approve or reject a pending request.
	PermissionReview = "idc:admin:compliance:tax-invoice:review"
	// PermissionSubmit allows staff to queue an approved request for provider submission.
	PermissionSubmit = "idc:admin:compliance:tax-invoice:submit"
	// PermissionCancel allows staff to safely cancel an unissued request.
	PermissionCancel = "idc:admin:compliance:tax-invoice:cancel"

	StatusPendingReview             = "pending_review"
	StatusApprovedPendingSubmission = "approved_pending_submission"
	StatusRejected                  = "rejected"
	StatusQueued                    = "queued"
	StatusFailed                    = "failed"
	StatusCanceled                  = "canceled"

	InvoiceTypePendingConfirmation  = "pending_confirmation"
	InvoiceTypeVATElectronicNormal  = "vat_electronic_normal"
	InvoiceTypeVATElectronicSpecial = "vat_electronic_special"
	InvoiceTypeDigitalNormal        = "digital_electronic_normal"
	InvoiceTypeDigitalSpecial       = "digital_electronic_special"

	ProviderNuoNuo                 = "nuonuo_invoice"
	CapabilityExternalConfirmation = "external_confirmation_required"
	CapabilityCheckUnavailable     = "capability_check_unavailable"

	ReasonBuyerOrTaxInformationIncorrect    = "buyer_or_tax_information_incorrect"
	ReasonAmountOrBillingAdjustmentRequired = "amount_or_billing_invoice_adjustment_required"
	ReasonDuplicateRequest                  = "duplicate_request"
	ReasonBusinessNotInvoiceable            = "business_not_invoiceable"
	ReasonCustomerRequestedCancellation     = "customer_requested_cancellation"
	ReasonOrderOrRefundChanged              = "order_or_refund_changed"
	ReasonOther                             = "other"
)

// Capability is the current provider-neutral invoice capability projection.
type Capability struct {
	ProviderCode          string
	Ready                 bool
	ReasonCode            string
	SupportedInvoiceTypes []string
}

// CapabilityGate reports whether this installation can safely use a provider.
type CapabilityGate interface {
	Current(context.Context) (Capability, error)
}

// ClosedNuoNuoGate keeps production submission closed until external evidence is accepted.
type ClosedNuoNuoGate struct{}

// Current returns the explicit external-evidence blocker without calling NuoNuo.
func (ClosedNuoNuoGate) Current(context.Context) (Capability, error) {
	return Capability{
		ProviderCode: ProviderNuoNuo,
		Ready:        false,
		ReasonCode:   CapabilityExternalConfirmation,
	}, nil
}

// Request is the safe staff projection of one tax-invoice request.
type Request struct {
	TaxInvoiceNo          string
	CustomerNo            string
	StatusCode            string
	InvoiceTypeCode       string
	ProviderCode          string
	DecisionReasonCode    string
	DecisionInternalNote  string
	GrossAmountMinor      int64
	NetAmountMinor        int64
	TaxAmountMinor        int64
	CurrencyCode          string
	MaskedInvoiceNumber   string
	Version               int64
	SubmittedAt           time.Time
	ReviewedAt            *time.Time
	IssuedAt              *time.Time
	DeliveredAt           *time.Time
	CanceledAt            *time.Time
	ProviderNotIssuedAt   *time.Time
	CapabilityReady       bool
	CapabilityReasonCode  string
	SupportedInvoiceTypes []string
}

// ListQuery controls one count-free staff directory page.
type ListQuery struct {
	Page       int
	Cursor     string
	CustomerNo string
	StatusCode string
	Limit      int
}

// Page is one bounded tax-invoice directory page.
type Page struct {
	Total      int
	Items      []Request
	NextCursor string
}

type Allocation struct {
	AllocationNo, BillingInvoiceNo, OrderNo          string
	GrossAmountMinor, NetAmountMinor, TaxAmountMinor int64
	RefundedGrossMinor, RedLetteredGrossMinor        int64
}

type Item struct {
	ItemNo, AllocationNo, BillingInvoiceItemNo, TaxClassificationCode string
	LineNo, TaxRatePPM                                                int
	GrossAmountMinor, NetAmountMinor, TaxAmountMinor                  int64
}

type File struct {
	FileNo, FileTypeCode, ContentSHA256, MediaType, StatusCode string
	ByteSize                                                   int64
	Immutable                                                  bool
	ArchivedAt                                                 *time.Time
	CreatedAt                                                  time.Time
}

// BuyerProfile is the immutable buyer profile selected when the customer
// submitted the request. The underlying secret handle and bank-account secret
// reference are deliberately not representable in this projection.
type BuyerProfile struct {
	ProfileNo, TitleTypeCode, InvoiceTitle, TaxRegistrationNo string
	RegisteredAddress, RegisteredPhone, BankName              string
	BankAccountMasked                                         string
	Version                                                   int64
}

// Detail contains bounded immutable financial and file evidence.
type Detail struct {
	Request          Request
	Buyer            BuyerProfile
	Allocations      []Allocation
	Items            []Item
	Files            []File
	HistoryTruncated bool
}

// ReviewInput approves a pending request with one currently supported invoice type.
type ReviewInput struct {
	TaxInvoiceNo    string
	InvoiceTypeCode string
	ExpectedVersion int64
	IdempotencyKey  string
}

// CloseInput rejects or cancels one request with a stable customer-visible reason.
type CloseInput struct {
	TaxInvoiceNo    string
	ReasonCode      string
	InternalNote    string
	ExpectedVersion int64
	IdempotencyKey  string
}

// SubmitInput queues one approved request without contacting the provider.
type SubmitInput struct {
	TaxInvoiceNo    string
	ExpectedVersion int64
	IdempotencyKey  string
}

// Result reports whether an action changed state or was safely blocked by capability.
type Result struct {
	Request *Request
	Applied bool
	Reused  bool
}

// Service exposes the four separately authorized management capabilities.
type Service interface {
	List(context.Context, ListQuery) (*Page, error)
	Get(context.Context, string) (*Request, error)
	GetDetail(context.Context, string) (*Detail, error)
	Approve(context.Context, ReviewInput) (*Result, error)
	Reject(context.Context, CloseInput) (*Result, error)
	Submit(context.Context, SubmitInput) (*Result, error)
	Cancel(context.Context, CloseInput) (*Result, error)
}

type serviceImpl struct {
	database         gdb.DB
	runtime          managementcap.Runtime
	gate             CapabilityGate
	snapshotAccessor secretcap.Accessor
	now              func() time.Time
}

var _ Service = (*serviceImpl)(nil)

// New constructs the installation-local management service.
func New(
	database gdb.DB,
	runtime managementcap.Runtime,
	gate CapabilityGate,
	snapshotAccessor secretcap.Accessor,
) (Service, error) {
	if nilDependency(database) || managementcap.ValidateRuntime(runtime) != nil ||
		nilDependency(gate) || nilDependency(snapshotAccessor) {
		return nil, gerror.New("tax invoice management dependencies are invalid")
	}
	return &serviceImpl{
		database: database, runtime: runtime, gate: gate,
		snapshotAccessor: snapshotAccessor, now: time.Now,
	}, nil
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
