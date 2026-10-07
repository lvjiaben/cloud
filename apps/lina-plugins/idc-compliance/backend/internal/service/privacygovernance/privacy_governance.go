// Package privacygovernance owns installation-local privacy assessments,
// retention, customer requests, incidents, and safe support exports.
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
	PermissionQuery     = "idc:admin:compliance:privacy:query"
	PermissionConfigure = "idc:admin:compliance:privacy:configure"
	PermissionIncident  = "idc:admin:compliance:privacy:incident"

	AssessmentPersonalVerification   AssessmentType = "personal_verification"
	AssessmentEnterpriseVerification AssessmentType = "enterprise_verification"
	AssessmentElectronicContract     AssessmentType = "electronic_contract"
	AssessmentTaxInvoice             AssessmentType = "tax_invoice"
	AssessmentLocalSensitiveExport   AssessmentType = "local_sensitive_export"
	AssessmentAISafeProjection       AssessmentType = "ai_safe_projection"

	AssessmentDraft      AssessmentStatus = "draft"
	AssessmentPublished  AssessmentStatus = "published"
	AssessmentSuperseded AssessmentStatus = "superseded"

	DataPersonalVerificationMaterial   DataType = "personal_verification_material"
	DataEnterpriseVerificationMaterial DataType = "enterprise_verification_material"
	DataElectronicContractEvidence     DataType = "electronic_contract_evidence"
	DataTaxInvoiceEvidence             DataType = "tax_invoice_evidence"
	DataSensitiveAccessAudit           DataType = "sensitive_access_audit"
	DataLocalSensitiveExport           DataType = "local_sensitive_export"
)

// AssessmentType is one of the six independently versioned assessments.
type AssessmentType string

// AssessmentStatus is the immutable publication lifecycle.
type AssessmentStatus string

// DataType is one independently configured retention category.
type DataType string

// Assessment contains governance text only. It must never carry personal data,
// object paths, tokens, credentials, or secret references.
type Assessment struct {
	AssessmentNo             string
	Type                     AssessmentType
	Version                  int64
	Status                   AssessmentStatus
	Purpose                  string
	Necessity                string
	Impact                   string
	Recipients               string
	StorageLocation          string
	RetentionDescription     string
	RightsEntry              string
	MerchantRole             string
	ThirdPartyRole           string
	ProcessingInstruction    string
	SubcontractingBoundary   string
	AuditEvidenceDescription string
	AIAllowedFields          []string
	CrossBorderTransfer      bool
	ConfirmedByUserID        int
	ConfirmedAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	RowVersion               int64
}

// AssessmentDraftInput saves one complete draft using optimistic concurrency.
// ExpectedRowVersion is zero only when creating the first draft for this type.
type AssessmentDraftInput struct {
	Type                     AssessmentType
	Purpose                  string
	Necessity                string
	Impact                   string
	Recipients               string
	StorageLocation          string
	RetentionDescription     string
	RightsEntry              string
	MerchantRole             string
	ThirdPartyRole           string
	ProcessingInstruction    string
	SubcontractingBoundary   string
	AuditEvidenceDescription string
	AIAllowedFields          []string
	ExpectedRowVersion       int64
}

// PublishAssessmentInput requires an explicit UI confirmation and replay key.
type PublishAssessmentInput struct {
	AssessmentNo       string
	ExpectedRowVersion int64
	Confirmed          bool
	IdempotencyKey     string
}

// AssessmentPage is one bounded history page without a total-count query.
type AssessmentPage struct {
	Items []Assessment
}

// RetentionPolicy is one current or superseded merchant-configured duration.
type RetentionPolicy struct {
	PolicyNo      string
	DataType      DataType
	Version       int64
	RetentionDays int
	StatusCode    string
	CreatedBy     int
	CreatedAt     time.Time
}

// RetentionPolicyInput replaces the current policy without hardcoded years.
type RetentionPolicyInput struct {
	DataType       DataType
	RetentionDays  int
	IdempotencyKey string
}

// CollectionDecision proves that both the assessment and retention policy are current.
type CollectionDecision struct {
	AssessmentNo   string
	PolicyNo       string
	RetentionUntil time.Time
}

// CollectionGate is injected into sensitive collection owners.
type CollectionGate interface {
	RequireCollection(context.Context, AssessmentType, DataType) (*CollectionDecision, error)
}

// RequireSensitiveCollection is the string-only injection seam used by
// sibling sensitive-data owners without importing privacy implementation types.
func (s *serviceImpl) RequireSensitiveCollection(
	ctx context.Context,
	assessmentTypeCode string,
	dataTypeCode string,
) (time.Time, error) {
	decision, err := s.RequireCollection(ctx, AssessmentType(assessmentTypeCode), DataType(dataTypeCode))
	if err != nil {
		return time.Time{}, err
	}
	return decision.RetentionUntil, nil
}

// CustomerNotice is one published collection notice plus its current retention duration.
type CustomerNotice struct {
	AssessmentType       AssessmentType
	AssessmentNo         string
	Version              int64
	Purpose              string
	Necessity            string
	Impact               string
	Recipients           string
	StorageLocation      string
	RetentionDescription string
	RetentionDays        int
	RightsEntry          string
}

// CustomerRequest is one ownership-only export or deletion request.
type CustomerRequest struct {
	RequestNo      string
	RequestType    string
	StatusCode     string
	HoldReasonCode string
	SubmittedAt    time.Time
	CompletedAt    *time.Time
	Version        int64
}

// CustomerPrivacyOverview is the complete bounded privacy-center projection.
type CustomerPrivacyOverview struct {
	Notices  []CustomerNotice
	Consents []identitycap.PrivacyConsentDecision
	Requests []CustomerRequest
}

// CustomerPrivacyRequestInput submits an export or delete request without free-form sensitive text.
type CustomerPrivacyRequestInput struct {
	RequestType    string
	IdempotencyKey string
}

// Incident is one installation-local response record. SafeSummary and timeline
// notes must never contain original personal material, object paths, or secrets.
type Incident struct {
	IncidentNo                    string
	StatusCode                    string
	SeverityCode                  string
	SourceCode                    string
	SafeSummary                   string
	TicketGeneration              int64
	ExportBlocked                 bool
	ExternalNotificationConfirmed bool
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	ResolvedAt                    *time.Time
	Version                       int64
	Timeline                      []IncidentEvent
}

// IncidentEvent is one immutable safe response-stage note.
type IncidentEvent struct {
	EventNo     string
	StageCode   string
	SafeNote    string
	ActorUserID int
	OccurredAt  time.Time
}

// IncidentCreateInput records a detected anomaly and immediately rotates access generation.
type IncidentCreateInput struct {
	SeverityCode   string
	SourceCode     string
	SafeSummary    string
	IdempotencyKey string
}

// IncidentAdvanceInput moves exactly one stage with optimistic concurrency.
type IncidentAdvanceInput struct {
	IncidentNo                  string
	ExpectedVersion             int64
	NextStageCode               string
	SafeNote                    string
	ConfirmExternalNotification bool
	IdempotencyKey              string
}

// SupportExport is a local, minimal, redacted diagnostics file and its receipt.
type SupportExport struct {
	ExportNo      string
	FileName      string
	Content       []byte
	ContentSHA256 string
	FieldCount    int
	RecordCount   int
	CreatedAt     time.Time
}

// SupportExportInput requires a separately agreed local handoff confirmation.
type SupportExportInput struct {
	SupportAgreementConfirmed bool
	IdempotencyKey            string
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

// RetentionHoldInput is the exact-target seam used by local contract, tax,
// dispute, audit, and legal owners. It never carries document content.
type RetentionHoldInput struct {
	TargetTypeCode string
	TargetNo       string
	ReasonCode     string
	OwnerCode      string
	IdempotencyKey string
}

// RetentionHoldReleaseInput releases only the exact active reason previously
// placed by a local owner.
type RetentionHoldReleaseInput struct {
	TargetTypeCode string
	TargetNo       string
	ReasonCode     string
	OwnerCode      string
	IdempotencyKey string
}

// Service exposes privacy governance operations. Additional Phase 10.13
// capabilities are implemented on this same owner so there is no central gateway.
type Service interface {
	ListAssessments(context.Context, AssessmentType, int) (*AssessmentPage, error)
	SaveAssessmentDraft(context.Context, AssessmentDraftInput) (*Assessment, error)
	PublishAssessment(context.Context, PublishAssessmentInput) (*Assessment, error)
	CurrentPublished(context.Context, AssessmentType) (*Assessment, error)
	ListRetentionPolicies(context.Context) ([]RetentionPolicy, error)
	SetRetentionPolicy(context.Context, RetentionPolicyInput) (*RetentionPolicy, error)
	RequireCollection(context.Context, AssessmentType, DataType) (*CollectionDecision, error)
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
	CurrentCustomerPrivacy(context.Context) (*CustomerPrivacyOverview, error)
	WithdrawCurrentCustomerConsent(context.Context, identitycap.PrivacyConsentWithdrawalInput) (*identitycap.PrivacyConsentDecision, error)
	CreateCurrentCustomerRequest(context.Context, CustomerPrivacyRequestInput) (*CustomerRequest, error)
	ListIncidents(context.Context, int) ([]Incident, error)
	CreateIncident(context.Context, IncidentCreateInput) (*Incident, error)
	AdvanceIncident(context.Context, IncidentAdvanceInput) (*Incident, error)
	CurrentSensitiveAccessGeneration(context.Context) (int64, bool, error)
	GenerateSupportExport(context.Context, SupportExportInput) (*SupportExport, error)
	ProjectAISafeFields(context.Context, map[string]any) (map[string]any, error)
	ScanDueDeletionTasks(context.Context, int) (int, error)
	GenerateDueDeletionTasks(context.Context, int) (int, error)
	ListDeletionTasks(context.Context, int) ([]DeletionTask, error)
	ConfirmDeletion(context.Context, DeletionConfirmInput) (*DeletionTask, error)
	PlaceRetentionHold(context.Context, RetentionHoldInput) error
	ReleaseRetentionHold(context.Context, RetentionHoldReleaseInput) error
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

// NewComplete adds physical storage and personal snapshot revocation for the
// deletion queue. Production route composition uses this constructor.
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

// NewWithCustomer adds the ownership-only customer privacy center to the same
// installation-local owner used by administration.
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

var _ Service = (*serviceImpl)(nil)

// New constructs the installation-local privacy governance owner.
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
