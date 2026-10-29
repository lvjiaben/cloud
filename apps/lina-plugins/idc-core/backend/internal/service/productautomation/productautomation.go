// Package productautomation owns safe administration of generic automation
// interfaces and the single immutable fulfillment binding of a product version.
package productautomation

import (
	"context"
	"crypto/rand"
	"encoding/json"
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
	ForProductBinding      bool
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
	CatalogAvailable    bool
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
	CatalogAvailable          bool
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
	CustomerFieldSelections []CustomerFieldSelection
	VersionNo               string
	MerchantValues          []ConfigurationValue
	ExpectedVersion         int64
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

// CustomerFieldSelection enables a declared optional field, a merchant-only local
// field, or presentation settings for a retained core field. Provider bounds stay authoritative.
type CustomerFieldSelection struct {
	CustomerVisible *bool           `json:"customerVisible,omitempty"`
	DefaultValue    json.RawMessage `json:"defaultValue,omitempty"`

	// UpgradeEnabled is nil for existing offers, retaining their previous upgrade behavior.
	UpgradeEnabled         *bool    `json:"upgradeEnabled,omitempty"`
	UpgradeMinimum         string   `json:"upgradeMinimum,omitempty"`
	UpgradeMaximum         string   `json:"upgradeMaximum,omitempty"`
	UpgradeCandidateValues []string `json:"upgradeCandidateValues,omitempty"`

	// ChoiceRules are owner-derived commercial restrictions, never accepted
	// as arbitrary provider instructions from an administration DTO.
	ChoiceRules              []*capmodel.ConfigurationRuleDeclaration `json:"-"`
	AutomationMappingPending bool                                     `json:"-"`
	BasePriceIncluded        bool                                     `json:"-"`
	FixedPriceIncluded       bool                                     `json:"-"`
	InformationOnly          bool                                     `json:"informationOnly,omitempty"`
	ProductChoiceBindings    map[string]string                        `json:"productChoiceBindings,omitempty"`
	ProductFieldKey          string                                   `json:"productFieldKey,omitempty" dc:"Explicit provisioning API parameter name; declared fields retain their type checks, custom native names and contents are frozen for direct provisioning; empty records customer information only" eg:"region"`
	NumericPricing           string                                   `json:"numericPricing,omitempty" dc:"Numeric pricing policy: quantity keeps unit or band prices, per_choice uses independent fixed specification prices; omitted preserves existing behavior" eg:"quantity"`
	GroupZhCN                string                                   `json:"groupZhCN,omitempty" dc:"Configuration group Chinese label" eg:"Specifications"`
	GroupEnUS                string                                   `json:"groupEnUS,omitempty" dc:"Configuration group English label" eg:"Specifications"`
	Unit                     string                                   `json:"unit,omitempty" dc:"Merchant display suffix without parameter conversion" eg:"GiB"`
	DescriptionZhCN          string                                   `json:"descriptionZhCN,omitempty" dc:"Merchant Chinese field description" eg:"Choose a specification"`
	DescriptionEnUS          string                                   `json:"descriptionEnUS,omitempty" dc:"Merchant English field description" eg:"Choose a specification"`
	Advanced                 bool                                     `json:"advanced,omitempty" dc:"Whether the purchase field is in advanced settings" eg:"false"`
	NumericLabels            map[string]*capmodel.LocalizedText       `json:"numericLabels,omitempty" dc:"Optional bilingual display labels keyed by numeric choices" eg:"{}"`
	LocalField               *capmodel.ConfigurationFieldDeclaration  `json:"localField,omitempty" dc:"Merchant-only typed field; reserved local_ key; never forwarded to providers" eg:"{}"`

	DisplayControl  string                             `json:"displayControl,omitempty"`
	QuantityStep    string                             `json:"quantityStep,omitempty"`
	NumericChoices  []string                           `json:"numericChoices,omitempty"`
	FieldKey        string                             `json:"fieldKey"`
	Minimum         string                             `json:"minimum,omitempty"`
	Maximum         string                             `json:"maximum,omitempty"`
	CandidateValues []string                           `json:"candidateValues,omitempty"`
	CandidateLabels map[string]*capmodel.LocalizedText `json:"candidateLabels,omitempty" dc:"Optional bilingual labels for the ordered merchant choice values" eg:"{}"`
	DisplayNameZhCN string                             `json:"displayNameZhCN,omitempty" dc:"Merchant Chinese display name; empty preserves the connector label" eg:"CPU cores"`
	DisplayNameEnUS string                             `json:"displayNameEnUS,omitempty" dc:"Merchant English display name; empty preserves the connector label" eg:"CPU cores"`
	SortOrder       int                                `json:"sortOrder,omitempty" dc:"Optional display order for enabled merchant fields, from zero to 1024" eg:"1"`
}

type VersionConfiguration struct {
	DirectoryRelationships     []resourceinterfaceprojectioncap.ChoiceRelationship
	MerchantPriceTargets       []MerchantPriceTarget
	ProductChoiceFields        []string
	CustomerProductFields      map[string]string
	PriceFieldTokens           map[string]string
	UpstreamOptionsStatus      resourceinterfaceprojectioncap.CandidateSyncState
	CustomerConfigurableFields []*capmodel.ConfigurationFieldDeclaration
	CustomerFieldSelections    []CustomerFieldSelection
	// ImportedConfiguration protects an immutable supplier source or portable contract.
	ImportedPricingMode        string
	ImportedMarkupPercent      string
	ImportedManualPrices       []resourceinterfaceprojectioncap.CatalogPrice
	ImportedConfiguration      bool
	ImportedProductName        string
	ImportedProductReference   string
	ProductDeclaration         *capmodel.ConfigurationSchemaDeclaration
	CustomerDeclaration        *capmodel.ConfigurationSchemaDeclaration
	MerchantValues             []ConfigurationValue
	ProductResolvedCandidates  configurationcontract.ResolvedCandidates
	CustomerResolvedCandidates configurationcontract.ResolvedCandidates
	Version                    int64
}

// MerchantPriceTarget belongs only to the authorized configuration editor.
// Unlike the separate opaque pricing API, this projection joins values the
// merchant is already editing to their existing local price dimensions.
type MerchantPriceTarget struct {
	FieldKey       string
	CandidateValue string
	Quantity       bool
	DimensionNo    string
	PricingMode    catalog.ConfigurationPriceMode
}

// MerchantPriceTargetInput identifies a price target in the just-saved merchant
// schema. Token is local to one atomic save, never a public supplier identity.
type MerchantPriceTargetInput struct {
	Token          string
	FieldKey       string
	CandidateValue string
	Quantity       bool
	PricingMode    catalog.ConfigurationPriceMode
}

// ConfigurationPriceCandidate is one safe, currently selectable customer
// candidate. CandidateToken does not reveal the plugin field key or value.
type ConfigurationPriceCandidate struct {
	FieldToken     string
	Quantity       bool
	PricingMode    catalog.ConfigurationPriceMode
	CandidateToken string
	FieldLabel     capmodel.LocalizedText
	CandidateLabel capmodel.LocalizedText
	DimensionNo    string
}

// ConfigurationPriceDimension is the merchant-created, version-frozen price
// target. Its plugin field identity remains private to the service.
type ConfigurationPriceDimension struct {
	PricingMode    catalog.ConfigurationPriceMode
	DimensionNo    string
	FieldLabel     capmodel.LocalizedText
	CandidateLabel capmodel.LocalizedText
}

type CreateConfigurationPriceDimensionInput struct {
	PricingMode    catalog.ConfigurationPriceMode
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
// catalog publication preview. It preserves a safe failure reason for unusable
// bindings and returns errors when readiness cannot be determined safely.
type PublicationReadiness interface {
	// CheckPublicationReadiness reuses the authoritative publication admission
	// evidence and lifecycle checks without changing catalog or interface state.
	CheckPublicationReadiness(ctx context.Context, versionNo string) (catalog.PublicationReadinessCheck, error)
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

// Service owns supplier-interface bindings, typed product configuration and
// frozen customer schemas. Merchant editing and public projections retain
// their separate authority; an existing product reference prevents deletion
// of the supplier identity used by historical orders.
type Service interface {
	// GetMerchantConfiguration checks exact configuration visibility and joins
	// one bounded local price-target projection for the editor. Internal reads
	// and public purchase projections retain GetVersionConfiguration semantics.
	GetMerchantConfiguration(context.Context, string) (*VersionConfiguration, error)
	// ResolveMerchantPriceTargets validates up to 256 targets against the saved
	// schema under the caller transaction, checks configuration authority, and
	// creates missing dimensions in one batch. The price owner checks price
	// authority before calling it; invalid targets or mode changes reject all.
	ResolveMerchantPriceTargets(context.Context, string, []MerchantPriceTargetInput) (map[string]string, error)
	// ResolvePriceTargets resolves at most 256 local fixed-price tokens under the caller transaction; exact configuration authorization, draft locking, revision checking and audit are enforced.
	ResolvePriceTargets(context.Context, string, []string) (map[string]string, error)
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
	// WithSaleStateAdmission checks published-provider admission when resuming;
	// stopping stays available offline and executes only the authorized local callback.
	WithSaleStateAdmission(context.Context, string, bool, func(context.Context) error) error
}

type serviceImpl struct {
	booleanPriceLabels         map[bool]capmodel.LocalizedText
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
	falseLabel capmodel.LocalizedText,
	trueLabel capmodel.LocalizedText,
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
	if falseLabel.ZhCN == "" || falseLabel.EnUS == "" || trueLabel.ZhCN == "" || trueLabel.EnUS == "" {
		return nil, gerror.New("boolean price labels are required")
	}
	service.booleanPriceLabels = map[bool]capmodel.LocalizedText{false: falseLabel, true: trueLabel}
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
