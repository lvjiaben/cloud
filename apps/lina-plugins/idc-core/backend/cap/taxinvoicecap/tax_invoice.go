// Package taxinvoicecap exposes the core-owned billing and invoice-profile
// truth needed by the compliance plugin. It is source-only and never accepts
// customer identity, tax amounts, tax rates, tax categories, or seller
// identity from a browser payload.
package taxinvoicecap

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// AllowedCallerPluginID is the only plugin allowed to load this source runtime.
	AllowedCallerPluginID = "idc-compliance"
	// MaximumProfiles bounds profile picker data returned in one call.
	MaximumProfiles = 50
	// MaximumInvoices bounds billing invoice options returned in one call.
	MaximumInvoices = 50
	// MaximumSelections bounds invoices selected in one request.
	MaximumSelections = 20
	// MaximumItemsPerRequest bounds item snapshots across one request.
	MaximumItemsPerRequest = 200
	// MaximumRefundOperations bounds one background intake page.
	MaximumRefundOperations = 50
)

// ProfileSummary is safe for a customer-facing profile picker. The bank
// secret reference is deliberately absent.
type ProfileSummary struct {
	ProfileNo         string
	TitleTypeCode     string
	InvoiceTitle      string
	TaxRegistrationNo string
	RegisteredAddress string
	RegisteredPhone   string
	BankName          string
	BankAccountMasked string
	IsDefault         bool
	Version           int64
}

// ProfileSnapshot is the exact active buyer profile selected for one request.
// BankAccountSecretRef remains an opaque core secret-envelope reference.
type ProfileSnapshot struct {
	ProfileSummary
	BankAccountSecretRef string
}

// MerchantSnapshot identifies the installation-local seller. The provider
// taxpayer binding remains owned by the local compliance gateway.
type MerchantSnapshot struct {
	MerchantNo  string
	LegalName   string
	CountryCode string
	Version     int64
}

// InvoiceOption is one current-customer paid billing invoice and its amount
// available before a new tax-invoice request is submitted.
type InvoiceOption struct {
	BillingInvoiceNo      string
	OrderNo               string
	CurrencyCode          string
	SourceGrossMinor      int64
	AlreadyAllocatedMinor int64
	RemainingGrossMinor   int64
	BillingInvoiceVersion int64
	BillingContentDigest  string
	SettledAt             time.Time
}

// RequestOptions is the bounded source projection used to render a request form.
type RequestOptions struct {
	CustomerNo string
	Profiles   []ProfileSummary
	Invoices   []InvoiceOption
	ObservedAt time.Time
}

// Selection is the only customer-controlled financial input. Tax facts are
// intentionally not representable here.
type Selection struct {
	BillingInvoiceNo    string
	RequestedGrossMinor int64
}

// ItemUsage is compliance-owned prior allocation usage for one immutable core
// billing item.
type ItemUsage struct {
	BillingInvoiceItemNo string
	AllocatedGrossMinor  int64
	AllocatedNetMinor    int64
	AllocatedTaxMinor    int64
}

// InvoiceUsage is the compliance-owned amount already reserved by existing
// tax-invoice requests. Items must add up to AllocatedGrossMinor.
type InvoiceUsage struct {
	BillingInvoiceNo    string
	AllocatedGrossMinor int64
	Items               []ItemUsage
}

// AllocationUsageReader is implemented by the compliance owner. The core
// calls it only with the already-proven current customer and bounded invoice
// numbers; implementations must use the ambient transaction when present.
type AllocationUsageReader interface {
	// ReadAllocatedUsage returns compliance-owned reservations for the bounded invoice list.
	ReadAllocatedUsage(context.Context, string, []string) ([]InvoiceUsage, error)
}

// ItemSnapshot is the server-calculated immutable tax line copied from the
// authoritative order or renewal price snapshot.
type ItemSnapshot struct {
	BillingInvoiceItemNo  string
	SourceReference       string
	DescriptionZhCN       string
	DescriptionEnUS       string
	TaxTreatmentCode      string
	TaxRatePPM            int
	TaxClassificationCode string
	GrossAmountMinor      int64
	NetAmountMinor        int64
	TaxAmountMinor        int64
	SourceContentDigest   string
}

// InvoiceSnapshot is one selected billing invoice allocation.
type InvoiceSnapshot struct {
	InvoiceOption
	RequestedGrossMinor int64
	NetAmountMinor      int64
	TaxAmountMinor      int64
	Items               []ItemSnapshot
}

// RequestSnapshot is the complete bounded input that the compliance owner
// seals before creating a pending-review request.
type RequestSnapshot struct {
	CustomerNo         string
	PrincipalReference string
	ActorUserID        int
	Buyer              ProfileSnapshot
	Seller             MerchantSnapshot
	CurrencyCode       string
	GrossAmountMinor   int64
	NetAmountMinor     int64
	TaxAmountMinor     int64
	Invoices           []InvoiceSnapshot
	ObservedAt         time.Time
}

// RefundAllocation is one effective, posted refund amount for an immutable
// core billing item. Compliance receives only opaque business references and
// cannot inspect core tables or alter the refund.
type RefundAllocation struct {
	RefundAllocationNo   string
	BillingInvoiceNo     string
	BillingInvoiceItemNo string
	AmountMinor          int64
	ContentDigest        string
}

// RefundOperation is the bounded posted-refund projection used to create
// supplier-neutral red-letter work after customer funds have already moved.
type RefundOperation struct {
	RefundOperationNo string
	CustomerNo        string
	CurrencyCode      string
	Allocations       []RefundAllocation
	PostedAt          time.Time
}

// PostedRefundReference is one ordered, opaque refund operation discovered by
// the compliance intake worker. Cursor is source-owned and never exposed over HTTP.
type PostedRefundReference struct {
	RefundOperationNo string
	Cursor            string
}

// PostedRefundFeed lists completed refund operations in stable source order.
// The consumer advances the cursor only after RecordPostedRefund commits.
type PostedRefundFeed interface {
	// ListPostedRefundOperations returns the next bounded page after an opaque source cursor.
	ListPostedRefundOperations(context.Context, string, int) ([]PostedRefundReference, error)
}

// Clear releases caller-owned slices after the compliance snapshot is sealed.
func (s *RequestSnapshot) Clear() {
	if s == nil {
		return
	}
	for index := range s.Invoices {
		clear(s.Invoices[index].Items)
		s.Invoices[index].Items = nil
	}
	clear(s.Invoices)
	s.Invoices = nil
}

// Service exposes only current-customer source truth.
type Service interface {
	// ListCurrentCustomerOptions returns only the authenticated customer's safe
	// profile projection and currently invoiceable paid billing invoices.
	ListCurrentCustomerOptions(context.Context, AllocationUsageReader) (*RequestOptions, error)
	// PrepareCurrentCustomerRequest locks the selected source rows and derives an
	// immutable server-owned snapshot; callers must seal it in the same workflow.
	PrepareCurrentCustomerRequest(
		context.Context,
		AllocationUsageReader,
		string,
		[]Selection,
	) (*RequestSnapshot, error)
	// GetPostedRefund returns one completed refund by opaque operation number.
	// It is available only inside the compliance-only startup runtime and never
	// accepts a customer identity or tax-invoice choice from a browser.
	GetPostedRefund(context.Context, string) (*RefundOperation, error)
}

// Runtime is the exact source-only graph supplied to compliance.
type Runtime struct {
	Source     Service
	RefundFeed PostedRefundFeed
}

// ValidateRuntime rejects absent and typed-nil dependencies.
func ValidateRuntime(runtime Runtime) error {
	if nilDependency(runtime.Source) || nilDependency(runtime.RefundFeed) {
		return gerror.New("tax invoice source runtime is incomplete")
	}
	return nil
}

// nilDependency detects absent runtime interfaces, including typed nil values.
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
