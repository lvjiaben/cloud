// Package electroniccontracttemplate owns installation-local contract-template management.
package electroniccontracttemplate

import (
	"context"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// PermissionQuery allows safe list, detail, and preview reads.
	PermissionQuery = "idc:admin:compliance:contract-template:query"
	// PermissionManage allows create, copy, edit, synchronization, enable, and disable.
	PermissionManage = "idc:admin:compliance:contract-template:manage"

	OriginLocal         = "local"
	OriginTencentSynced = "tencent_synced"

	ProviderStateActive   = "active"
	ProviderStateDisabled = "disabled"
	ProviderStateOffline  = "offline"

	StatusDraft                 = "draft"
	StatusNeedsConfiguration    = "needs_configuration"
	StatusSubmissionUnavailable = "submission_unavailable"
	StatusSubmitted             = "submitted"
	StatusEnabled               = "enabled"
	StatusDisabled              = "disabled"
	StatusProviderDisabled      = "provider_disabled"
	StatusProviderOffline       = "provider_offline"

	ScopePersonal     = "personal"
	ScopeEnterprise   = "enterprise"
	ScopeAllCustomers = "all_customers"

	SigningOrderCustomerFirst = "customer_first"
	SigningOrderMerchantFirst = "merchant_first"
	SigningOrderParallel      = "parallel"

	LanguageZhCN = "zh-CN"
	LanguageEnUS = "en-US"

	PageSize = 20

	SourceModeUploadedFile = "uploaded_file"
	SourceModeOnlineEditor = "online_editor"

	ConversionSourceSaved = "source_saved"
	ConversionReady       = "ready"

	FieldTypeTextVariable   = "text_variable"
	FieldTypeSignature      = "signature"
	FieldTypeEnterpriseSeal = "enterprise_seal"
	FieldTypeSigningDate    = "signing_date"

	PartyCustomer = "customer"
	PartyMerchant = "merchant"

	VariableContractNo   = "contract_no"
	VariableCustomerName = "customer_name"
	VariableMerchantName = "merchant_name"
	VariableOrderNo      = "order_no"
	VariableServiceName  = "service_name"
	VariableServiceTerm  = "service_term"
	VariableAmount       = "amount"
	VariableCurrency     = "currency"
	VariableSigningDate  = "signing_date"

	ProviderValueContractNo   = "contract_no"
	ProviderValueCustomerName = "customer_name"
	ProviderValueMerchantName = "merchant_name"
	ProviderValueOrderNo      = "order_no"
	ProviderValueServiceLines = "service_lines"
	ProviderValueTotalAmount  = "total_amount"
	ProviderValueCurrency     = "currency"
	ProviderValueSigningDate  = "signing_date"

	ProviderRecipientIndividual = "INDIVIDUAL"
	ProviderRecipientEnterprise = "ENTERPRISE"
	ProviderRecipientPromoter   = "PROMOTER"
)

// ListInput selects one fixed-size management page.
type ListInput struct {
	PageSize   int
	Name       string
	ScopeCode  string
	StatusCode string
	Page       int
}

// Item is the safe logical-template latest-version projection.
type Item struct {
	TemplateNo       string
	VersionNo        int64
	Name             string
	ScopeCode        string
	SigningOrderCode string
	LanguageCode     string
	StatusCode       string
	OriginCode       string
	ProviderState    string
	ProviderVersion  string
	UsedCount        int64
	LockVersion      int64
	UpdatedAt        time.Time
}

// List is one exact offset page.
type List struct {
	Items    []Item
	Page     int
	PageSize int
	Total    int64
}

// CreateInput creates the first local draft version.
type CreateInput struct {
	Name             string
	ScopeCode        string
	SigningOrderCode string
	LanguageCode     string
	IdempotencyKey   string
}

// CopyInput copies safe metadata into a new local draft without copying provider-owned body bytes.
type CopyInput struct {
	SourceTemplateNo string
	SourceVersionNo  int64
	IdempotencyKey   string
}

// UpdateInput changes only mutable draft metadata or completes synchronized local settings.
type UpdateInput struct {
	TemplateNo       string
	VersionNo        int64
	Name             string
	ScopeCode        string
	SigningOrderCode string
	LanguageCode     string
	ExpectedVersion  int64
}

// StateInput applies one explicit local enable or disable transition.
type StateInput struct {
	TemplateNo      string
	VersionNo       int64
	ExpectedVersion int64
}

// SyncResult summarizes one complete explicit provider snapshot application.
type SyncResult struct {
	ProviderTotal int
	NewVersions   int
	MarkedOffline int
	SyncedAt      time.Time
}

// ContentRevision is a safe append-only source/PDF projection.
type ContentRevision struct {
	ContentRevisionNo    string
	RevisionNo           int64
	SourceModeCode       string
	SourceMediaType      string
	SourceByteSize       int64
	SourceSHA256         string
	ConversionStatusCode string
	PDFSHA256            string
	PDFByteSize          int64
	PDFPageCount         int
	CreatedAt            time.Time
}

// FieldPlacement is one normalized position in millionths of page dimensions.
type FieldPlacement struct {
	FieldNo       string
	SequenceNo    int
	FieldTypeCode string
	PartyCode     string
	VariableCode  string
	PageNo        int
	PositionXPPM  int
	PositionYPPM  int
	WidthPPM      int
	HeightPPM     int
}

// ProviderComponent is one safe Tencent fill or signing-date position and its confirmed mapping.
type ProviderComponent struct {
	ComponentRef string
	Name         string
	TypeCode     string
	Required     bool
	RecipientRef string
	ValueCode    string
}

// ProviderRecipient is one safe Tencent signer role and its confirmed local party mapping.
type ProviderRecipient struct {
	RecipientRef       string
	SequenceNo         int
	TypeCode           string
	RoleName           string
	RequireSign        bool
	RoutingOrder       *int64
	IsPromoter         bool
	SignComponentCount int
	PartyCode          string
}

// ProviderFieldMapping maps one exact Tencent position to one fixed contract value.
type ProviderFieldMapping struct {
	ComponentRef string
	ValueCode    string
}

// ProviderRecipientMapping maps one exact Tencent signer role to customer or merchant.
type ProviderRecipientMapping struct {
	RecipientRef string
	PartyCode    string
}

// SaveProviderMappingInput confirms the complete mapping for one exact provider snapshot.
type SaveProviderMappingInput struct {
	TemplateNo        string
	VersionNo         int64
	ExpectedVersion   int64
	IdempotencyKey    string
	FieldMappings     []ProviderFieldMapping
	RecipientMappings []ProviderRecipientMapping
}

// EditorDetail combines safe metadata, the current content revision, and fields.
type EditorDetail struct {
	Item
	Content              *ContentRevision
	Fields               []FieldPlacement
	ProviderComponents   []ProviderComponent
	ProviderRecipients   []ProviderRecipient
	ProviderMappingSaved bool
	OnlineHTML           string
	PreviewConfirmed     bool
	EnableReady          bool
	EnableReasonCode     string
	SubmissionAvailable  bool
	SubmissionReasonCode string
}

// UploadedSourceInput streams one bounded source without a browser filename.
type UploadedSourceInput struct {
	TemplateNo        string
	VersionNo         int64
	ExpectedVersion   int64
	DeclaredMediaType string
	IdempotencyKey    string
	Body              io.Reader
}

// OnlineSourceInput saves one bounded online-editor HTML source.
type OnlineSourceInput struct {
	TemplateNo      string
	VersionNo       int64
	ExpectedVersion int64
	IdempotencyKey  string
	HTML            string
}

// GeneratePreviewInput converts the exact current source into an immutable PDF revision.
type GeneratePreviewInput struct {
	TemplateNo        string
	VersionNo         int64
	ExpectedVersion   int64
	ContentRevisionNo string
	IdempotencyKey    string
}

// ConfirmPreviewInput records an explicit administrator confirmation for the exact current PDF.
type ConfirmPreviewInput struct {
	TemplateNo        string
	VersionNo         int64
	ExpectedVersion   int64
	ContentRevisionNo string
}

// SaveFieldSetInput appends one complete placement set for the current PDF.
type SaveFieldSetInput struct {
	TemplateNo        string
	VersionNo         int64
	ExpectedVersion   int64
	ContentRevisionNo string
	IdempotencyKey    string
	Fields            []FieldPlacement
}

// PreviewContent is a bounded decrypted PDF cleared by the controller.
type PreviewContent struct {
	Content []byte
	SHA256  string
}

// Clear releases the decrypted in-process preview bytes as soon as the controller has written them.
func (c *PreviewContent) Clear() {
	if c == nil {
		return
	}
	clearContentBytes(c.Content)
	c.Content = nil
}

// Service exposes only the frozen management workflow.
type Service interface {
	List(context.Context, ListInput) (*List, error)
	Get(context.Context, string, int64) (*Item, error)
	Create(context.Context, CreateInput) (*Item, error)
	Copy(context.Context, CopyInput) (*Item, error)
	Update(context.Context, UpdateInput) (*Item, error)
	Enable(context.Context, StateInput) (*Item, error)
	Disable(context.Context, StateInput) (*Item, error)
	SyncTencent(context.Context) (*SyncResult, error)
	GetEditor(context.Context, string, int64) (*EditorDetail, error)
	SaveUploadedSource(context.Context, UploadedSourceInput) (*EditorDetail, error)
	SaveOnlineSource(context.Context, OnlineSourceInput) (*EditorDetail, error)
	GeneratePreview(context.Context, GeneratePreviewInput) (*EditorDetail, error)
	ConfirmPreview(context.Context, ConfirmPreviewInput) (*EditorDetail, error)
	OpenPreview(context.Context, string, int64, string) (*PreviewContent, error)
	OpenProviderPreview(context.Context, string, int64) (string, error)
	SaveFieldSet(context.Context, SaveFieldSetInput) (*EditorDetail, error)
	SaveProviderMapping(context.Context, SaveProviderMappingInput) (*EditorDetail, error)
}

type tencentGateway interface {
	TencentTemplates(context.Context) (localgatewayclient.TencentTemplateList, error)
}

type tencentPreviewGateway interface {
	TencentTemplatePreview(context.Context, string) (string, error)
}

type serviceImpl struct {
	database    gdb.DB
	runtime     managementcap.Runtime
	gateway     tencentGateway
	storage     storagecap.Service
	keyAccessor secretcap.Accessor
	keyManager  secretcap.GeneratedManager
	converter   DocumentConverter
	now         func() time.Time
}

var _ Service = (*serviceImpl)(nil)

// New constructs one installation-scoped template management service.
func New(database gdb.DB, runtime managementcap.Runtime, gateway tencentGateway) (Service, error) {
	if nilDependency(database) || managementcap.ValidateRuntime(runtime) != nil || nilDependency(gateway) {
		return nil, gerror.New("electronic contract template dependencies are invalid")
	}
	return &serviceImpl{database: database, runtime: runtime, gateway: gateway, now: time.Now}, nil
}

// NewWithContent constructs the complete metadata and encrypted-content service.
func NewWithContent(
	database gdb.DB,
	runtime managementcap.Runtime,
	gateway tencentGateway,
	storage storagecap.Service,
	keyAccessor secretcap.Accessor,
	keyManager secretcap.Manager,
	converter DocumentConverter,
) (Service, error) {
	generated, ok := keyManager.(secretcap.GeneratedManager)
	if nilDependency(storage) || nilDependency(keyAccessor) || !ok || nilDependency(generated) ||
		nilDependency(converter) {
		return nil, gerror.New("electronic contract template content dependencies are invalid")
	}
	base, err := New(database, runtime, gateway)
	if err != nil {
		return nil, err
	}
	service, ok := base.(*serviceImpl)
	if !ok {
		return nil, gerror.New("electronic contract template base implementation is invalid")
	}
	service.storage = storage
	service.keyAccessor = keyAccessor
	service.keyManager = generated
	service.converter = converter
	return service, nil
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
