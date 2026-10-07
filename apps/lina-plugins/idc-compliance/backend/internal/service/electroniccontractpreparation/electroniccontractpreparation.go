// Package electroniccontractpreparation freezes one immutable, encrypted
// contract preview for each order before any Tencent flow is created.
package electroniccontractpreparation

import (
	"context"
	"crypto/rand"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/service/contractlifecycle"
	"lina-plugin-idc-compliance/backend/internal/service/contractsigner"
	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	ProviderTencentESSBasic     = "tencent_essbasic"
	SigningValidity             = 7 * 24 * time.Hour
	PermissionContractCreate    = "idc:admin:compliance:contract:create"
	PermissionContractQuery     = "idc:admin:compliance:contract:query"
	PermissionContractReissue   = "idc:admin:compliance:contract:reissue"
	PermissionContractTerminate = "idc:admin:compliance:contract:terminate"
)

// PrepareInput reserves one contract for a required-contract order.
type PrepareInput struct {
	OrderNo        string
	SignerNo       string
	IdempotencyKey string
}

// ManualOptionalInput starts a non-blocking contract for an order whose
// frozen business decision is that no contract is required.
type ManualOptionalInput struct {
	OrderNo        string
	TemplateNo     string
	SignerNo       string
	IdempotencyKey string
}

// SubmitSigningInput submits one already-created immutable preview.
type SubmitSigningInput struct {
	ContractNo      string
	ExpectedVersion int64
	IdempotencyKey  string
}

// PrepareReplacementInput creates a new immutable preview linked to one failed contract.
type PrepareReplacementInput struct {
	ContractNo     string
	TemplateNo     string
	SignerNo       string
	IdempotencyKey string
}

// ReissueSigningInput submits an unchanged contract or one prepared replacement.
type ReissueSigningInput struct {
	ContractNo      string
	ExpectedVersion int64
	IdempotencyKey  string
}

// TerminateContractInput submits a Tencent release agreement for a completed contract.
type TerminateContractInput struct {
	ContractNo                string
	ExpectedVersion           int64
	Reason                    string
	RemainInForceItem         string
	OriginalExpenseSettlement string
	OriginalOtherSettlement   string
	OtherDeals                string
	Confirmed                 bool
	IdempotencyKey            string
}

// SubmittedSigning is the safe result of one guarded provider launch.
type SubmittedSigning struct {
	ContractNo string
	FlowNo     string
	StatusCode string
	Version    int64
	Reused     bool
}

// ContractFlowGateway is the installation-local signed Tencent write seam.
type ContractFlowGateway interface {
	// CreateTencentContractFlow submits one frozen contract flow with a stable business idempotency key.
	CreateTencentContractFlow(context.Context, processauth.TencentContractFlowRequest, string) (processauth.TencentContractFlowResponse, error)
	// CreateTencentContractTermination submits one frozen release agreement without accepting supplier identifiers.
	CreateTencentContractTermination(context.Context, processauth.TencentContractTerminationRequest, string) (processauth.TencentContractFlowResponse, error)
	// CreateTencentContractSigningLink creates one short-lived link for an existing flow without relaunching it.
	CreateTencentContractSigningLink(context.Context, processauth.TencentContractSigningLinkRequest) (processauth.TencentContractSigningLinkResponse, error)
	// RecheckTencentContractFlow performs one supplier-authoritative read for an existing flow.
	RecheckTencentContractFlow(context.Context, processauth.TencentContractFlowRecheckRequest) (processauth.TencentContractFlowRecheckResponse, error)
}

// CustomerContractQuery is one bounded stable-cursor list request.
type CustomerContractQuery struct {
	Cursor string
	Limit  int
}

// CustomerContract is the customer-safe projection shared by list and order detail.
type CustomerContract struct {
	ContractNo       string
	OrderNo          string
	TemplateName     string
	LanguageCode     string
	AmountMinor      int64
	CurrencyCode     string
	StatusCode       string
	CreatedAt        time.Time
	SigningExpiresAt time.Time
	CanOpenSigning   bool
	CanRecheck       bool
	Version          int64
}

// CustomerContractPage is one bounded page without an expensive exact total.
type CustomerContractPage struct {
	Items      []CustomerContract
	NextCursor string
}

// CustomerSigningLink is a non-persisted short-lived Tencent H5 link.
type CustomerSigningLink struct {
	Contract  CustomerContract
	URL       string
	ExpiresAt time.Time
}

// ManagementContractQuery is one bounded contract directory request.
type ManagementContractQuery struct {
	Page       int
	StatusCode string
	OrderNo    string
	CustomerNo string
	Cursor     string
	PageSize   int
}

// ManagementContractListItem is the secret-safe contract directory projection.
type ManagementContractListItem struct {
	ContractNo             string
	CustomerNo             string
	OrderNo                string
	ContractTypeCode       string
	RequiredForFulfillment bool
	TemplateNo             string
	TemplateVersion        int64
	TemplateName           string
	LanguageCode           string
	SubjectTypeCode        string
	SigningOrderCode       string
	AmountMinor            int64
	CurrencyCode           string
	StatusCode             string
	LaunchedAt             *time.Time
	CompletedAt            *time.Time
	TerminatedAt           *time.Time
	SigningExpiresAt       *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
}

// ManagementContractFlow is one installation-local signing or termination attempt.
type ManagementContractFlow struct {
	FlowNo         string
	SequenceNo     int
	PurposeCode    string
	StatusCode     string
	ProviderStatus string
	FailureCode    string
	ExpiresAt      *time.Time
	SubmittedAt    *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
}

// ManagementContractEvent is one append-only supplier-authoritative timeline item.
type ManagementContractEvent struct {
	FlowNo             string
	SourceKind         string
	ProviderStatus     string
	ProjectedStatus    string
	DecisionCode       string
	ReasonCode         string
	SupplierObservedAt time.Time
	ObservedAt         time.Time
	CreatedAt          time.Time
}

// ManagementContractFile is safe immutable file metadata without a storage reference.
type ManagementContractFile struct {
	FileNo             string
	FlowNo             string
	FileTypeCode       string
	ContentSHA256      string
	MediaType          string
	ByteSize           int64
	StatusCode         string
	Immutable          bool
	ArchivedAt         *time.Time
	RetentionUntil     *time.Time
	LegalHold          bool
	SupplierObservedAt *time.Time
	CreatedAt          time.Time
}

// ManagementContractArchiveJob is one bounded artifact collection status.
type ManagementContractArchiveJob struct {
	JobNo              string
	FlowNo             string
	ArtifactTypeCode   string
	StatusCode         string
	AttemptCount       int
	LastReasonCode     string
	SupplierObservedAt *time.Time
	CompletedAt        *time.Time
	NextAttemptAt      *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ManagementContractReplacement preserves one old-to-new immutable contract relation.
type ManagementContractReplacement struct {
	ReplacementNo string
	OldContractNo string
	NewContractNo string
	CreatedAt     time.Time
}

// ManagementContractPage is one count-free keyset page.
type ManagementContractPage struct {
	Total      int
	Items      []ManagementContractListItem
	NextCursor string
}

// ManagementContractDetail is one bounded local contract evidence projection.
type ManagementContractDetail struct {
	Item             ManagementContractListItem
	Flows            []ManagementContractFlow
	Events           []ManagementContractEvent
	Files            []ManagementContractFile
	ArchiveJobs      []ManagementContractArchiveJob
	Replacements     []ManagementContractReplacement
	HistoryTruncated bool
}

// ManualOptionalTemplate is one enabled template that is safe for the exact
// optional-contract order. It intentionally excludes document content and
// supplier identifiers.
type ManualOptionalTemplate struct {
	TemplateNo       string
	VersionNo        int64
	Name             string
	ScopeCode        string
	SigningOrderCode string
	LanguageCode     string
}

// ManualOptionalOptions identifies the exact order owner and eligible templates.
type ManualOptionalOptions struct {
	OrderNo      string
	CustomerNo   string
	CustomerType contractcap.CustomerType
	Templates    []ManualOptionalTemplate
}

// ReplacementSigner is one verified enterprise handler selectable for a new contract version.
type ReplacementSigner struct {
	SignerNo    string
	MaskedName  string
	MaskedPhone string
}

// ReplacementOptions contains templates and signers eligible for one failed contract.
type ReplacementOptions struct {
	ContractNo   string
	OrderNo      string
	CustomerNo   string
	CustomerType contractcap.CustomerType
	Templates    []ManualOptionalTemplate
	Signers      []ReplacementSigner
}

// PreparedContract is the bounded customer-visible pre-sign projection.
// Clear must be called after the response or provider request is written.
type PreparedContract struct {
	ContractNo             string
	OrderNo                string
	CustomerNo             string
	CustomerType           contractcap.CustomerType
	Template               contractcap.TemplateSnapshot
	SigningOrderCode       string
	AmountMinor            int64
	CurrencyCode           string
	CreatedAt              time.Time
	SigningExpiresAt       time.Time
	RequiredForFulfillment bool
	Preview                *contractcap.PreviewVariables
	SignerNo               string
	SignerName             []byte
	SignerPhoneMasked      string
	Reused                 bool
	Version                int64
}

// Clear releases the decrypted preview names and line items.
func (p *PreparedContract) Clear() {
	if p == nil {
		return
	}
	if p.Preview != nil {
		p.Preview.Clear()
		p.Preview = nil
	}
	clear(p.SignerName)
	p.SignerName = nil
}

// Service owns contract preparation without launching a provider flow.
type Service interface {
	// PrepareRequired freezes one contract required by the order policy without launching a provider flow.
	PrepareRequired(context.Context, PrepareInput) (*PreparedContract, error)
	// ListManualOptionalTemplates returns the bounded templates eligible for an optional order contract.
	ListManualOptionalTemplates(context.Context, string) ([]ManualOptionalTemplate, error)
	// GetManualOptionalOptions returns the safe order owner/type and eligible templates in one authorization pass.
	GetManualOptionalOptions(context.Context, string) (*ManualOptionalOptions, error)
	// PrepareManualOptional freezes one administrator-confirmed optional contract without launching it.
	PrepareManualOptional(context.Context, ManualOptionalInput) (*PreparedContract, error)
	// SubmitSigning launches exactly one previously frozen contract through the guarded local gateway.
	SubmitSigning(context.Context, SubmitSigningInput) (*SubmittedSigning, error)
	// PrepareReplacement freezes a changed contract preview and append-only replacement relation without launching Tencent.
	PrepareReplacement(context.Context, PrepareReplacementInput) (*PreparedContract, error)
	// GetReplacementOptions returns exact templates and verified signers for a changed reissue preview.
	GetReplacementOptions(context.Context, string) (*ReplacementOptions, error)
	// SubmitReissue launches a fresh seven-day reissue for an unchanged contract or prepared replacement.
	SubmitReissue(context.Context, ReissueSigningInput) (*SubmittedSigning, error)
	// TerminateContract submits one confirmed release agreement while preserving the completed contract.
	TerminateContract(context.Context, TerminateContractInput) (*SubmittedSigning, error)
	// ListManagementContracts returns one bounded merchant-local page after exact view authorization.
	ListManagementContracts(context.Context, ManagementContractQuery) (*ManagementContractPage, error)
	// GetManagementContract returns bounded flows, events, files, and archive states for one visible contract.
	GetManagementContract(context.Context, string) (*ManagementContractDetail, error)
	// MatchCustomerSigner verifies that an observed signer still matches the frozen customer identity.
	MatchCustomerSigner(context.Context, string, *contractcap.ObservedCustomerSigner) error
	// ValidateAuthoritativeSigningDate accepts a final signing date only from the Tencent authority result.
	ValidateAuthoritativeSigningDate(string, time.Time) error
	// ListCurrentContracts returns one bounded local page owned by the authenticated customer.
	ListCurrentContracts(context.Context, CustomerContractQuery) (*CustomerContractPage, error)
	// CurrentOrderContract returns the authenticated customer's unique contract for one order.
	CurrentOrderContract(context.Context, string) (*CustomerContract, error)
	// CreateCurrentSigningLink creates a fresh 30-minute link for the existing signable flow only.
	CreateCurrentSigningLink(context.Context, string) (*CustomerSigningLink, error)
	// RecheckCurrentContract reads Tencent authoritatively and applies only a monotonic safe projection.
	RecheckCurrentContract(context.Context, string) (*CustomerContract, error)
}

type serviceImpl struct {
	database         gdb.DB
	runtime          contractcap.Runtime
	accessor         secretcap.Accessor
	manager          secretcap.Manager
	templates        contracttemplatecap.Source
	authorizer       managementcap.Authorizer
	artifactTexts    ArtifactTextCatalog
	signers          contractsigner.Service
	random           randomReader
	gateway          ContractFlowGateway
	lifecycle        contractlifecycle.Service
	customers        identitycap.CurrentCustomerReader
	signingReturnURL string
	privacyGate      privacyCollectionGate
}

type privacyCollectionGate interface {
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
}

type randomReader interface {
	Read([]byte) (int, error)
}

var _ Service = (*serviceImpl)(nil)

// New constructs the installation-local contract preparation service.
func New(
	database gdb.DB,
	runtime contractcap.Runtime,
	accessor secretcap.Accessor,
	manager secretcap.Manager,
	templates contracttemplatecap.Source,
	authorizer managementcap.Authorizer,
	artifactTexts ArtifactTextCatalog,
	signerServices ...contractsigner.Service,
) (Service, error) {
	return newService(database, runtime, accessor, manager, templates, authorizer, artifactTexts, nil, signerServices...)
}

// NewWithGateway constructs the preparation and guarded launch service graph.
func NewWithGateway(
	database gdb.DB,
	runtime contractcap.Runtime,
	accessor secretcap.Accessor,
	manager secretcap.Manager,
	templates contracttemplatecap.Source,
	authorizer managementcap.Authorizer,
	artifactTexts ArtifactTextCatalog,
	gateway ContractFlowGateway,
	lifecycle contractlifecycle.Service,
	customers identitycap.CurrentCustomerReader,
	signingReturnURL string,
	signerServices ...contractsigner.Service,
) (Service, error) {
	if nilDependency(gateway) || nilDependency(lifecycle) || nilDependency(customers) || !validSigningReturnURL(signingReturnURL) {
		return nil, gerror.New("electronic contract flow gateway is invalid")
	}
	implementation, err := newService(database, runtime, accessor, manager, templates, authorizer, artifactTexts, gateway, signerServices...)
	if err != nil {
		return nil, err
	}
	implementation.customers = customers
	implementation.lifecycle = lifecycle
	implementation.signingReturnURL = signingReturnURL
	implementation.privacyGate = openPrivacyCollectionGate{}
	return implementation, nil
}

// NewWithGatewayAndPrivacyGate applies the published assessment and retention
// policy before a new immutable contract snapshot is collected.
func NewWithGatewayAndPrivacyGate(
	database gdb.DB,
	runtime contractcap.Runtime,
	accessor secretcap.Accessor,
	manager secretcap.Manager,
	templates contracttemplatecap.Source,
	authorizer managementcap.Authorizer,
	artifactTexts ArtifactTextCatalog,
	gateway ContractFlowGateway,
	lifecycle contractlifecycle.Service,
	customers identitycap.CurrentCustomerReader,
	signingReturnURL string,
	privacyGate privacyCollectionGate,
	signerServices ...contractsigner.Service,
) (Service, error) {
	service, err := NewWithGateway(database, runtime, accessor, manager, templates, authorizer, artifactTexts, gateway, lifecycle, customers, signingReturnURL, signerServices...)
	if err != nil {
		return nil, err
	}
	implementation, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("electronic contract preparation implementation is unavailable")
	}
	if nilDependency(privacyGate) {
		return nil, gerror.New("electronic contract privacy gate is invalid")
	}
	implementation.privacyGate = privacyGate
	return implementation, nil
}

type openPrivacyCollectionGate struct{}

func (openPrivacyCollectionGate) RequireSensitiveCollection(context.Context, string, string) (time.Time, error) {
	return time.Now().UTC().Add(24 * time.Hour), nil
}

func newService(
	database gdb.DB,
	runtime contractcap.Runtime,
	accessor secretcap.Accessor,
	manager secretcap.Manager,
	templates contracttemplatecap.Source,
	authorizer managementcap.Authorizer,
	artifactTexts ArtifactTextCatalog,
	gateway ContractFlowGateway,
	signerServices ...contractsigner.Service,
) (*serviceImpl, error) {
	if nilDependency(database) || contractcap.ValidateRuntime(runtime) != nil || nilDependency(accessor) ||
		nilDependency(manager) || nilDependency(templates) || nilDependency(authorizer) || !artifactTexts.valid() {
		return nil, gerror.New("electronic contract preparation dependencies are invalid")
	}
	if len(signerServices) > 1 || len(signerServices) == 1 && nilDependency(signerServices[0]) {
		return nil, gerror.New("electronic contract signer directory is invalid")
	}
	var signers contractsigner.Service
	if len(signerServices) == 1 {
		signers = signerServices[0]
	}
	return &serviceImpl{
		database: database, runtime: runtime, accessor: accessor, manager: manager,
		templates: templates, authorizer: authorizer, artifactTexts: artifactTexts,
		signers: signers, random: rand.Reader,
		gateway: gateway,
	}, nil
}

// ValidateAuthoritativeSigningDate accepts final dates only from Tencent's
// authoritative result, never from a browser return or local clock.
func (*serviceImpl) ValidateAuthoritativeSigningDate(providerCode string, signedAt time.Time) error {
	if providerCode != ProviderTencentESSBasic || signedAt.IsZero() {
		return gerror.New("final contract signing date is not provider authoritative")
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
