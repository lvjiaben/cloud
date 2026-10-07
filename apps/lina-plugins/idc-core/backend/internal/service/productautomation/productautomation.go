// Package productautomation owns safe administration of generic automation
// interfaces and the single immutable fulfillment binding of a product version.
package productautomation

import (
	"context"
	"crypto/rand"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const (
	defaultPageSize         = 20
	maximumPageSize         = 100
	InterfaceStateAccepting = "accepting"
	InterfaceStateStopped   = "stopped"
)

// Authorization is trusted server-side evidence returned by an action-specific
// authorizer. RequiredPermission is copied into audit and never comes from input.
type Authorization struct {
	ActorUserID        int
	RequiredPermission string
}

// Authorizer checks one exact management action before any host or database read.
// Implementations must hide absent and out-of-scope targets behind the same error.
type Authorizer interface {
	// AuthorizeProviderCandidates authorizes one bounded provider discovery read.
	AuthorizeProviderCandidates(ctx context.Context) (Authorization, error)
	// AuthorizeInterfaceList authorizes one bounded local interface read.
	AuthorizeInterfaceList(ctx context.Context) (Authorization, error)
	// AuthorizeInterfaceCreate authorizes creation and returns trusted audit evidence.
	AuthorizeInterfaceCreate(ctx context.Context) (Authorization, error)
	// AuthorizeInterfaceRename authorizes a future-facing name change while hiding inaccessible targets.
	AuthorizeInterfaceRename(ctx context.Context, interfaceNo string) (Authorization, error)
	// AuthorizeInterfaceDelete authorizes deletion eligibility checks while hiding inaccessible targets.
	AuthorizeInterfaceDelete(ctx context.Context, interfaceNo string) (Authorization, error)
	// AuthorizeInterfaceStop authorizes stopping new business for one interface.
	AuthorizeInterfaceStop(ctx context.Context, interfaceNo string) (Authorization, error)
	// AuthorizeInterfaceResume authorizes resuming new business for one interface.
	AuthorizeInterfaceResume(ctx context.Context, interfaceNo string) (Authorization, error)
	// AuthorizeInterfaceConfiguration authorizes reading or changing provider credentials.
	AuthorizeInterfaceConfiguration(ctx context.Context, interfaceNo string) (Authorization, error)
	// AuthorizeBindingRead authorizes reading one visible product version's binding.
	AuthorizeBindingRead(ctx context.Context, versionNo string) (Authorization, error)
	// AuthorizeBindingWrite authorizes binding changes for one visible product version.
	AuthorizeBindingWrite(ctx context.Context, versionNo string) (Authorization, error)
	// AuthorizeVersionConfiguration authorizes reading or changing product configuration.
	AuthorizeVersionConfiguration(ctx context.Context, versionNo string) (Authorization, error)
	// AuthorizePublicationAdmission authorizes publication of one visible version.
	AuthorizePublicationAdmission(ctx context.Context, versionNo string) (Authorization, error)
}

// ProviderSelectionCodec creates and verifies purpose-isolated, bounded opaque
// selection tokens. Implementations must authenticate every evidence field and
// enforce their own expiry or token-version policy.
type ProviderSelectionCodec interface {
	// EncodeProviderSelection authenticates bounded host evidence for later selection.
	EncodeProviderSelection(ctx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) (string, error)
	// DecodeProviderSelection verifies purpose, integrity, size, and expiry before returning evidence.
	DecodeProviderSelection(ctx context.Context, token string) (*capmodel.BusinessExtensionProviderEvidence, error)
}

// PageInput requests one bounded, one-based page.
type PageInput struct {
	Page                   int
	PageSize               int
	NamePrefix             string
	NewBusinessState       string
	ProviderSelectionToken string
}

// ProviderCandidate is a safe UI projection. Provider identity, methods, and
// digests remain inside the authenticated selection token.
type ProviderCandidate struct {
	SelectionToken         string
	DisplayName            string
	ReleaseVersion         string
	ContractVersion        string
	FulfillmentCapability  *capmodel.LocalizedText
	InterfaceConfiguration *capmodel.ConfigurationSchemaDeclaration
}

// ProviderCandidatePage is one bounded provider discovery page.
type ProviderCandidatePage struct {
	Items []ProviderCandidate
	Total int
}

// InterfaceInstance is a safe UI projection of one immutable local interface.
type InterfaceInstance struct {
	InterfaceInstanceNo string
	DisplayName         string
	ReleaseVersion      string
	ContractVersion     string
	NewBusinessState    string
	Version             int64
	CreatedAt           time.Time
	ChangedAt           time.Time
	CanDelete           bool
}

// InterfacePage is one bounded local interface page.
type InterfacePage struct {
	Items []InterfaceInstance
	Total int
}

// Binding is the safe historical projection of one product-version binding.
type Binding struct {
	VersionNo                 string
	InterfaceInstanceNo       string
	InterfaceDisplayName      string
	ReleaseVersion            string
	ContractVersion           string
	BoundAt                   time.Time
	EvidenceVerified          bool
	InterfaceNewBusinessState string
}

// CreateInterfaceInput creates one immutable interface from an authenticated
// provider selection. DisplayName is merchant-authored and is not translated.
type CreateInterfaceInput struct {
	DisplayName    string
	SelectionToken string
	Configuration  []ConfigurationValue
}

// RenameInterfaceInput changes only the future-facing display name and requires
// the caller's last observed version for optimistic concurrency.
type RenameInterfaceInput struct {
	InterfaceInstanceNo string
	DisplayName         string
	ExpectedVersion     int64
}

// InterfaceStateInput requests accepting or stopped for future business and
// requires the caller's last observed version; it does not disable history.
type InterfaceStateInput struct {
	InterfaceInstanceNo string
	NewBusinessState    string
	ExpectedVersion     int64
}

// DeleteInterfaceInput identifies one mistaken interface and the last observed
// version. Deletion remains forbidden after any product-version reference.
type DeleteInterfaceInput struct {
	InterfaceInstanceNo string
	ExpectedVersion     int64
}

// BindInput binds or replaces the interface of one draft product version.
type BindInput struct {
	VersionNo           string
	InterfaceInstanceNo string
}

// SaveVersionConfigurationInput stores merchant values and freezes the exact
// provider product/customer declarations on one draft product version.
type SaveVersionConfigurationInput struct {
	VersionNo       string
	MerchantValues  []ConfigurationValue
	ExpectedVersion int64
}

type UpdateInterfaceConfigurationInput struct {
	InterfaceInstanceNo string
	Values              []ConfigurationValue
	ExpectedRevision    int64
}

type EvaluateInterfaceConfigurationInput struct {
	InterfaceInstanceNo string
	Values              []ConfigurationValue
	ExpectedRevision    int64
}

type EvaluateVersionConfigurationInput struct {
	VersionNo       string
	MerchantValues  []ConfigurationValue
	ExpectedVersion int64
}

type ManagementConfigurationFileInput struct {
	Surface        ConfigurationSurface
	TargetNo       string
	FieldKey       string
	FileName       string
	SizeBytes      int64
	TargetRevision int64
}

type ManagementConfigurationFileItem struct {
	FileToken string
	FileName  string
	MediaType string
	Status    string
	SizeBytes int64
	ExpiresAt time.Time
}

// ConfigurationSecretStatus is safe to return to administrators. It never
// contains plaintext or an opaque secret handle.
type ConfigurationSecretStatus struct {
	FieldKey   string
	Configured bool
	MaskedTail string
	UpdatedAt  time.Time
}

type InterfaceConfiguration struct {
	Declaration        *capmodel.ConfigurationSchemaDeclaration
	Values             []ConfigurationValue
	Secrets            []ConfigurationSecretStatus
	ResolvedCandidates configurationcontract.ResolvedCandidates
	Revision           int64
}

type VersionConfiguration struct {
	ProductDeclaration         *capmodel.ConfigurationSchemaDeclaration
	CustomerDeclaration        *capmodel.ConfigurationSchemaDeclaration
	MerchantValues             []ConfigurationValue
	ProductResolvedCandidates  configurationcontract.ResolvedCandidates
	CustomerResolvedCandidates configurationcontract.ResolvedCandidates
	Version                    int64
}

// ConfigurationPriceCandidate is one safe, currently selectable customer
// candidate. CandidateToken does not reveal the plugin field key or value.
type ConfigurationPriceCandidate struct {
	CandidateToken string
	FieldLabel     capmodel.LocalizedText
	CandidateLabel capmodel.LocalizedText
	DimensionNo    string
}

// ConfigurationPriceDimension is the merchant-created, version-frozen price
// target. Its plugin field identity remains private to the service.
type ConfigurationPriceDimension struct {
	DimensionNo    string
	FieldLabel     capmodel.LocalizedText
	CandidateLabel capmodel.LocalizedText
}

type CreateConfigurationPriceDimensionInput struct {
	VersionNo      string
	CandidateToken string
}

type ConfigurationFieldState struct {
	Visible  bool
	Required bool
	Disabled bool
}

type ConfigurationIssue struct {
	FieldKey string
	Code     string
}

type ConfigurationEvaluation struct {
	Values     []ConfigurationValue
	States     map[string]ConfigurationFieldState
	Candidates configurationcontract.ResolvedCandidates
	Issues     []ConfigurationIssue
	Valid      bool
	Revision   int64
}

// PublicationReadiness is the narrow read-only admission seam consumed by
// catalog publication preview. It returns false for a currently unusable
// binding and an error only when readiness cannot be determined safely.
type PublicationReadiness interface {
	// CheckPublicationReadiness reuses the authoritative publication admission
	// evidence and lifecycle checks without changing catalog or interface state.
	CheckPublicationReadiness(ctx context.Context, versionNo string) (bool, error)
}

// PublicPurchaseAvailability is the authorization-free internal seam used by
// the anonymous catalog after it has already resolved one published version.
type PublicPurchaseAvailability interface {
	CheckPublicPurchaseAvailability(ctx context.Context, versionNo string) (bool, error)
}

// PublicPurchaseCandidateResolver resolves dynamic customer fields through the
// exact frozen provider evidence instead of accepting provider identity from a
// public caller.
type PublicPurchaseCandidateResolver interface {
	ResolvePublicPurchaseCandidates(
		ctx context.Context,
		versionNo string,
		schema *capmodel.ConfigurationSchemaDeclaration,
		values []configurationcontract.ConfigurationValue,
	) (configurationcontract.ResolvedCandidates, error)
}

type ExistingBusinessRuntimeContextInput struct {
	PluginID, ReleaseVersion, Point, ContractVersion, ReleaseDigest, DeclarationDigest string
	Method, InterfaceInstanceNo                                                        string
	InterfaceConfigurationRevision                                                     int64
	IntentNo, OrderNo, OrderItemNo, ConfigurationSnapshotNo                            string
}

type ExistingBusinessRuntimeContext struct {
	ContextID string
	Release   func()
}

type ExistingBusinessRuntimeContextIssuer interface {
	IssueExistingBusinessRuntimeContext(context.Context, ExistingBusinessRuntimeContextInput) (*ExistingBusinessRuntimeContext, error)
}

// Service exposes the safe 0.9 automation-management subset. It permits only
// future-facing rename and new-business state changes, and permanently blocks
// deletion after the first product-version reference. Structured configuration
// editing remains outside this version.
type Service interface {
	ImportBindingService
	PublicationReadiness
	PublicPurchaseAvailability
	PublicPurchaseCandidateResolver
	ExistingBusinessRuntimeContextIssuer
	catalog.FulfillmentContractReader
	// ListProviderCandidates returns a bounded safe projection from one host list call.
	ListProviderCandidates(ctx context.Context, page PageInput) (*ProviderCandidatePage, error)
	// CreateInterface creates an immutable admitted interface and audit atomically.
	CreateInterface(ctx context.Context, input CreateInterfaceInput) (*InterfaceInstance, error)
	// ListInterfaces returns a bounded local projection without per-row host calls.
	ListInterfaces(ctx context.Context, page PageInput) (*InterfacePage, error)
	// RenameInterface changes the display name for future bindings only and returns
	// a conflict when the optimistic version is stale.
	RenameInterface(ctx context.Context, input RenameInterfaceInput) (*InterfaceInstance, error)
	// UpdateNewBusinessState stops or resumes future use; resume revalidates the
	// frozen plugin publication, extension, contract, and method evidence.
	UpdateNewBusinessState(ctx context.Context, input InterfaceStateInput) (*InterfaceInstance, error)
	// DeleteInterface removes only an interface that has never been referenced and
	// records the successful deletion in the same transaction.
	DeleteInterface(ctx context.Context, input DeleteInterfaceInput) error
	// StopNewBusiness prevents future binding and publication through this interface.
	StopNewBusiness(ctx context.Context, interfaceNo string) error
	// ResumeNewBusiness revalidates frozen host evidence before accepting new work.
	ResumeNewBusiness(ctx context.Context, interfaceNo string) error
	// Bind creates or replaces the interface of one draft version under admission.
	Bind(ctx context.Context, input BindInput) (*Binding, error)
	// Unbind removes the binding only while the visible version remains a draft.
	Unbind(ctx context.Context, versionNo string) error
	// GetBinding returns nil for an unbound visible version and exact-validates history.
	GetBinding(ctx context.Context, versionNo string) (*Binding, error)
	// SaveVersionConfiguration validates and freezes plugin declarations and
	// merchant values while the product version remains a locked draft.
	SaveVersionConfiguration(ctx context.Context, input SaveVersionConfigurationInput) error
	GetInterfaceConfiguration(ctx context.Context, interfaceNo string) (*InterfaceConfiguration, error)
	EvaluateInterfaceConfiguration(ctx context.Context, input EvaluateInterfaceConfigurationInput) (*ConfigurationEvaluation, error)
	UpdateInterfaceConfiguration(ctx context.Context, input UpdateInterfaceConfigurationInput) (*InterfaceConfiguration, error)
	GetVersionConfiguration(ctx context.Context, versionNo string) (*VersionConfiguration, error)
	ListConfigurationPriceCandidates(ctx context.Context, versionNo string) ([]ConfigurationPriceCandidate, error)
	CreateConfigurationPriceDimension(ctx context.Context, input CreateConfigurationPriceDimensionInput) (*ConfigurationPriceDimension, error)
	EvaluateVersionConfiguration(ctx context.Context, input EvaluateVersionConfigurationInput) (*ConfigurationEvaluation, error)
	ReserveManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput) (*ManagementConfigurationFileItem, error)
	UploadManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput, token, declaredType string, body io.Reader) (*ManagementConfigurationFileItem, error)
	GetManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput, token string) (*ManagementConfigurationFileItem, error)
	RevokeManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput, token string) (*ManagementConfigurationFileItem, error)
	// WithPublicationAdmission runs publication inside the transaction holding all
	// relevant version, binding, and interface locks.
	WithPublicationAdmission(ctx context.Context, versionNo string, action func(context.Context) error) error
}

type serviceImpl struct {
	database                   gdb.DB
	extensions                 pluginhost.BusinessExtensionRegistrar
	authorizer                 Authorizer
	audit                      idccap.BusinessAuditWriter
	codec                      ProviderSelectionCodec
	random                     io.Reader
	secrets                    secretcap.Manager
	secretAccessor             secretcap.Accessor
	storage                    storagecap.Service
	fileScanner                purchaseconfigurationfile.Scanner
	consumedFiles              purchaseconfigurationfile.ConsumedFileReader
	dynamicCandidates          *dynamicCandidateCache
	runtimeContexts            *supplierRuntimeContextStore
	runtimeFiles               *supplierRuntimeFileStore
	runtimeConfigurationLoader func(context.Context, *capmodel.BusinessExtensionProviderEvidence, dynamicCandidateScope) (*supplierinterfacecap.ConfigurationView, error)
	outboundClientFactory      supplierOutboundClientFactory
	resourceCandidates         resourceinterfaceprojectioncap.CandidateSource
}

var _ Service = (*serviceImpl)(nil)

// New constructs the service with explicit runtime dependencies.
func New(
	database gdb.DB,
	extensions pluginhost.BusinessExtensionRegistrar,
	authorizer Authorizer,
	audit idccap.BusinessAuditWriter,
	codec ProviderSelectionCodec,
) (Service, error) {
	return newWithRandom(database, extensions, authorizer, audit, codec, rand.Reader)
}

// NewConfigured constructs the full service including owner-scoped secret
// storage required by structured interface configuration writes.
func NewConfigured(
	database gdb.DB,
	extensions pluginhost.BusinessExtensionRegistrar,
	authorizer Authorizer,
	audit idccap.BusinessAuditWriter,
	codec ProviderSelectionCodec,
	secrets secretcap.Manager,
	secretAccessor secretcap.Accessor,
	storage storagecap.Service,
	fileScanner purchaseconfigurationfile.Scanner,
	resourceCandidates resourceinterfaceprojectioncap.CandidateSource,
) (Service, error) {
	service, err := newWithRandom(database, extensions, authorizer, audit, codec, rand.Reader)
	if err != nil {
		return nil, err
	}
	if nilDependency(secrets) || nilDependency(secretAccessor) || nilDependency(storage) || nilDependency(fileScanner) {
		return nil, gerror.New("product automation structured configuration requires secret and governed private file dependencies")
	}
	service.secrets = secrets
	service.secretAccessor = secretAccessor
	service.storage = storage
	service.fileScanner = fileScanner
	service.resourceCandidates = resourceCandidates
	service.consumedFiles, err = purchaseconfigurationfile.NewConsumedFileReader(database, storage)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func newWithRandom(
	database gdb.DB,
	extensions pluginhost.BusinessExtensionRegistrar,
	authorizer Authorizer,
	audit idccap.BusinessAuditWriter,
	codec ProviderSelectionCodec,
	random io.Reader,
) (*serviceImpl, error) {
	for _, dependency := range []any{database, extensions, authorizer, audit, codec, random} {
		if nilDependency(dependency) {
			return nil, gerror.New("product automation requires database, extension, authorization, audit, codec, and random dependencies")
		}
	}
	return &serviceImpl{
		database: database, extensions: extensions, authorizer: authorizer, audit: audit,
		codec: codec, random: random, dynamicCandidates: newDynamicCandidateCache(),
		runtimeContexts: newSupplierRuntimeContextStore(random), runtimeFiles: newSupplierRuntimeFileStore(),
		outboundClientFactory: newSupplierOutboundClient,
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
