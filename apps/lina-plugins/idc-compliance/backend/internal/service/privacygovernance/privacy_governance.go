// Package privacygovernance retains authentication and tax evidence lifecycles and customer consent.
// Management is embedded in identity verification; assessments, incidents and support exports are retired.
package privacygovernance

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	PermissionQuery                             = "idc:admin:compliance:privacy:query"
	PermissionConfigure                         = "idc:admin:compliance:privacy:configure"
	DataPersonalVerificationMaterial   DataType = "personal_verification_material"
	DataEnterpriseVerificationMaterial DataType = "enterprise_verification_material"
	DataTaxInvoiceEvidence             DataType = "tax_invoice_evidence"
)

type DataType string

type RetentionPolicy struct {
	PolicyNo      string
	DataType      DataType
	Version       int64
	RetentionDays int
	StatusCode    string
	CreatedBy     int
	CreatedAt     time.Time
}

type RetentionPolicyInput struct {
	DataType       DataType
	RetentionDays  int
	IdempotencyKey string
}

type CustomerPrivacyOverview struct {
	Consents []identitycap.PrivacyConsentDecision
}

type DeletionTask struct {
	TaskNo            string
	DataTypeCode      string
	TargetTypeCode    string
	TargetNo          string
	StatusCode        string
	DueAt             time.Time
	HoldReasonCode    string
	ConfirmedByUserID int
	ConfirmedAt       *time.Time
	CompletedAt       *time.Time
	Version           int64
}

type DeletionConfirmInput struct {
	TaskNo          string
	ExpectedVersion int64
	Confirmed       bool
	IdempotencyKey  string
}

type RetentionHoldInput struct {
	TargetTypeCode string
	TargetNo       string
	ReasonCode     string
	OwnerCode      string
	IdempotencyKey string
}

type RetentionHoldReleaseInput struct {
	TargetTypeCode string
	TargetNo       string
	ReasonCode     string
	OwnerCode      string
	IdempotencyKey string
}

type serviceImpl struct {
	database          gdb.DB
	runtime           managementcap.Runtime
	now               func() time.Time
	customers         identitycap.CurrentCustomerReader
	consents          identitycap.PrivacyConsentService
	customerAudit     idccap.BusinessAuditWriter
	storage           storagecap.Service
	personalSnapshots secretcap.Manager
}

// Service preserves explicit retention, legal holds and manually confirmed removal.
type Service interface {
	ListRetentionPolicies(context.Context) ([]RetentionPolicy, error)
	SetRetentionPolicy(context.Context, RetentionPolicyInput) (*RetentionPolicy, error)
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
	CurrentCustomerPrivacy(context.Context) (*CustomerPrivacyOverview, error)
	WithdrawCurrentCustomerConsent(context.Context, identitycap.PrivacyConsentWithdrawalInput) (*identitycap.PrivacyConsentDecision, error)
	ScanDueDeletionTasks(context.Context, int) (int, error)
	GenerateDueDeletionTasks(context.Context, int) (int, error)
	ListDeletionTasks(context.Context, int, int) (*DeletionPage, error)
	ConfirmDeletion(context.Context, DeletionConfirmInput) (*DeletionTask, error)
	PlaceRetentionHold(context.Context, RetentionHoldInput) error
	ReleaseRetentionHold(context.Context, RetentionHoldReleaseInput) error
}

func NewComplete(
	database gdb.DB,
	runtime managementcap.Runtime,
	customers identitycap.CurrentCustomerReader,
	consents identitycap.PrivacyConsentService,
	customerAudit idccap.BusinessAuditWriter,
	storage storagecap.Service,
	personalSnapshots secretcap.Manager,
) (Service, error) {
	service, err := NewWithCustomer(database, runtime, customers, consents, customerAudit)
	if err != nil {
		return nil, err
	}
	implementation, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("privacy governance implementation is unavailable")
	}
	if nilDependency(storage) || nilDependency(personalSnapshots) {
		return nil, gerror.New("privacy deletion dependencies are invalid")
	}
	implementation.storage = storage
	implementation.personalSnapshots = personalSnapshots
	return implementation, nil
}

func NewWithCustomer(
	database gdb.DB,
	runtime managementcap.Runtime,
	customers identitycap.CurrentCustomerReader,
	consents identitycap.PrivacyConsentService,
	customerAudit idccap.BusinessAuditWriter,
) (Service, error) {
	service, err := New(database, runtime)
	if err != nil {
		return nil, err
	}
	implementation, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("privacy governance implementation is unavailable")
	}
	if nilDependency(customers) || nilDependency(consents) || nilDependency(customerAudit) {
		return nil, gerror.New("customer privacy governance dependencies are invalid")
	}
	implementation.customers = customers
	implementation.consents = consents
	implementation.customerAudit = customerAudit
	return implementation, nil
}

func New(database gdb.DB, runtime managementcap.Runtime) (Service, error) {
	if nilDependency(database) || managementcap.ValidateRuntime(runtime) != nil {
		return nil, gerror.New("privacy governance dependencies are invalid")
	}
	return &serviceImpl{database: database, runtime: runtime, now: time.Now}, nil
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

// DeletionPage contains a bounded page and its exact authorized total.
type DeletionPage struct {
	Items []DeletionTask
	Total int
}
