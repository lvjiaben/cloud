// Package contractcap exposes the core-owned order and verified-identity
// snapshot needed by the compliance source plugin before it prepares a
// contract. The boundary is source-only: raw legal identity never crosses an
// HTTP or dynamic-plugin bridge.
package contractcap

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

var contractNoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

const (
	// AllowedCallerPluginID is the only source plugin allowed to consume this boundary.
	AllowedCallerPluginID = "idc-compliance"
	// MaximumOrderItems bounds one immutable order snapshot.
	MaximumOrderItems = 100
)

// CustomerType is the closed customer-signing subject vocabulary.
type CustomerType string

const (
	CustomerTypeIndividual CustomerType = "individual"
	CustomerTypeEnterprise CustomerType = "enterprise"
)

// OrderItem is one immutable commercial line copied from the authoritative order.
type OrderItem struct {
	OrderItemNo         string
	ProductNo           string
	ProductNameZhCN     string
	ProductNameEnUS     string
	ContractProductName string
	Quantity            int
	PurchaseCycles      int
	PeriodUnitCode      string
	PeriodCount         int
	RequiresContract    bool
	AmountMinor         int64
}

// TemplateSnapshot freezes the exact enabled template selected by the order.
type TemplateSnapshot struct {
	TemplateNo       string
	VersionNo        int64
	Name             string
	LanguageCode     string
	ScopeCode        string
	SigningOrderCode string
}

// MerchantIdentity is the installation-local legal merchant signer.
type MerchantIdentity struct {
	MerchantNo  string
	LegalName   []byte
	CountryCode string
	Version     int64
}

func (i *MerchantIdentity) Clear() {
	if i == nil {
		return
	}
	clear(i.LegalName)
	i.LegalName = nil
}

// PreviewItem is one separately rendered order line. The product name is
// already selected in the template language.
type PreviewItem struct {
	OrderItemNo    string
	ServiceName    string
	Quantity       int
	PurchaseCycles int
	PeriodUnitCode string
	PeriodCount    int
	AmountMinor    int64
}

// PreviewVariables is the complete pre-sign contract input. SigningDate stays
// empty until Tencent returns its authoritative final signing timestamp.
type PreviewVariables struct {
	ContractNo            string
	CustomerName          []byte
	MerchantName          []byte
	OrderNo               string
	Items                 []PreviewItem
	AmountMinor           int64
	CurrencyCode          string
	SigningDate           string
	SigningDateNoticeCode string
}

func (v *PreviewVariables) Clear() {
	if v == nil {
		return
	}
	clear(v.CustomerName)
	v.CustomerName = nil
	clear(v.MerchantName)
	v.MerchantName = nil
	clear(v.Items)
	v.Items = nil
}

// VerifiedIdentity is the current authoritative legal subject. Sensitive byte
// slices are caller-owned and must be cleared after the compliance snapshot is sealed.
type VerifiedIdentity struct {
	CustomerType                 CustomerType
	CustomerNo                   string
	LegalName                    []byte
	SubjectLookupDigest          string
	VerificationCaseNo           string
	VerificationVersion          int64
	VerifiedAt                   time.Time
	ExpiresAt                    *time.Time
	VerifiedPhone                []byte
	PhoneContactNo               string
	PhoneContactVersion          int64
	PhoneVerifiedAt              time.Time
	RegistrationCountryCode      string
	RegistrationIdentifier       []byte
	RegistrationIdentifierMasked string
}

// ObservedCustomerSigner is the provider-authoritative customer subject that
// must match the current verified core identity immediately before launch.
// The matcher consumes and clears every raw identity buffer.
type ObservedCustomerSigner struct {
	CustomerType                   CustomerType
	LegalName                      []byte
	PersonalIdentityNumber         []byte
	RegistrationCountryCode        string
	RegistrationIdentifier         []byte
	EnterpriseHandlerAuthorization bool
}

// Clear releases provider-returned raw signer identity buffers.
func (s *ObservedCustomerSigner) Clear() {
	if s == nil {
		return
	}
	clear(s.LegalName)
	s.LegalName = nil
	clear(s.PersonalIdentityNumber)
	s.PersonalIdentityNumber = nil
	clear(s.RegistrationIdentifier)
	s.RegistrationIdentifier = nil
}

// Clear releases sensitive in-process identity buffers.
func (i *VerifiedIdentity) Clear() {
	if i == nil {
		return
	}
	clear(i.LegalName)
	i.LegalName = nil
	clear(i.RegistrationIdentifier)
	i.RegistrationIdentifier = nil
	clear(i.VerifiedPhone)
	i.VerifiedPhone = nil
}

// OrderSnapshot is the bounded business truth consumed by contract preparation.
type OrderSnapshot struct {
	OrderNo            string
	StatusCode         string
	CurrencyCode       string
	AmountMinor        int64
	RequiresContract   bool
	OrderVersion       int64
	Template           TemplateSnapshot
	Merchant           MerchantIdentity
	Customer           VerifiedIdentity
	Items              []OrderItem
	SnapshotObservedAt time.Time
}

// Clear releases every sensitive field owned by the snapshot.
func (s *OrderSnapshot) Clear() {
	if s == nil {
		return
	}
	s.Customer.Clear()
	s.Merchant.Clear()
	clear(s.Items)
	s.Items = nil
}

// BuildPreviewVariables validates the full immutable snapshot and returns the
// structured pre-sign preview input without inventing a signing date.
func (s *OrderSnapshot) BuildPreviewVariables(contractNo string) (*PreviewVariables, error) {
	contractNo = strings.TrimSpace(contractNo)
	if s == nil || !contractNoPattern.MatchString(contractNo) ||
		s.OrderNo == "" || s.OrderVersion < 1 || s.AmountMinor < 0 ||
		s.CurrencyCode == "" || len(s.Customer.LegalName) == 0 ||
		len(s.Merchant.LegalName) == 0 || s.Merchant.MerchantNo == "" ||
		s.Merchant.Version < 1 || len(s.Items) == 0 ||
		(s.Template.LanguageCode != "zh-CN" && s.Template.LanguageCode != "en-US") ||
		s.Template.TemplateNo == "" || s.Template.VersionNo < 1 || s.Template.Name == "" {
		return nil, gerror.New("contract preview variables are incomplete")
	}
	items := make([]PreviewItem, 0, len(s.Items))
	var total int64
	for _, item := range s.Items {
		if item.OrderItemNo == "" || strings.TrimSpace(item.ContractProductName) == "" ||
			item.Quantity < 1 || item.PurchaseCycles < 1 || item.PeriodCount < 1 ||
			item.PeriodUnitCode == "" || item.AmountMinor < 0 || total > s.AmountMinor-item.AmountMinor {
			return nil, gerror.New("contract preview item variables are incomplete")
		}
		total += item.AmountMinor
		items = append(items, PreviewItem{
			OrderItemNo: item.OrderItemNo, ServiceName: item.ContractProductName,
			Quantity: item.Quantity, PurchaseCycles: item.PurchaseCycles,
			PeriodUnitCode: item.PeriodUnitCode, PeriodCount: item.PeriodCount,
			AmountMinor: item.AmountMinor,
		})
	}
	if total != s.AmountMinor {
		return nil, gerror.New("contract preview item total does not equal order total")
	}
	return &PreviewVariables{
		ContractNo: contractNo, CustomerName: append([]byte(nil), s.Customer.LegalName...),
		MerchantName: append([]byte(nil), s.Merchant.LegalName...), OrderNo: s.OrderNo,
		Items: items, AmountMinor: s.AmountMinor, CurrencyCode: s.CurrencyCode,
		SigningDate: "", SigningDateNoticeCode: "filled_after_authoritative_signature",
	}, nil
}

// Service reads one repeatable, bounded order and current verified identity snapshot.
type Service interface {
	PrepareOrder(context.Context, string) (*OrderSnapshot, error)
	PrepareOptionalOrder(context.Context, string) (*OrderSnapshot, error)
	MatchCustomerSigner(context.Context, VerifiedIdentity, *ObservedCustomerSigner) error
}

// FailureCompensationInput is the immutable cross-owner command issued only
// after one required contract reaches a supplier-authoritative terminal state.
type FailureCompensationInput struct {
	ContractNo       string
	OrderNo          string
	TerminalEventNo  string
	TerminalStatus   string
	PolicyNo         string
	PolicyVersion    int64
	HandlingDeadline time.Time
	TriggerCode      string
}

// FailureCompensationResult is the core-owned order and refund handoff. A
// refund_pending result means the order is stopped and the unique refund
// request is ready for the payment owner; it never means money was returned.
type FailureCompensationResult struct {
	CompensationNo  string
	RefundRequestNo string
	OrderNo         string
	StatusCode      string
	ReasonCode      string
	Reused          bool
}

// FailureCompensator atomically stops untouched fulfillment, releases
// inventory, and creates one payment-owner refund handoff. Unsafe or unknown
// state is preserved and returned as manual_review.
type FailureCompensator interface {
	CompensateContractFailure(context.Context, FailureCompensationInput) (*FailureCompensationResult, error)
}

// Runtime is the exact source-only contract graph supplied to compliance.
type Runtime struct {
	Orders        Service
	Compensations FailureCompensator
	Audit         idccap.BusinessAuditWriter
}

// ValidateRuntime rejects absent and typed-nil dependencies.
func ValidateRuntime(runtime Runtime) error {
	if nilDependency(runtime.Orders) || nilDependency(runtime.Audit) {
		return gerror.New("contractcap source runtime is incomplete")
	}
	return nil
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
