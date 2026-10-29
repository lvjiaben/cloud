// This file defines staff-only catalog draft management contracts.

package catalog

import (
	"context"
	"crypto/rand"
	"io"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	permissionCatalogCategoryCreate  = "idc:admin:catalog:category:create"
	permissionCatalogCategoryList    = "idc:admin:catalog:category:list"
	permissionCatalogCategoryUpdate  = "idc:admin:catalog:category:update"
	permissionCatalogCategoryDelete  = "idc:admin:catalog:category:delete"
	permissionCatalogCategoryRestore = "idc:admin:catalog:category:restore"
	permissionCatalogProductCreate   = "idc:admin:catalog:product:create"
	permissionCatalogProductDelete   = "idc:admin:catalog:product:delete"
	permissionCatalogProductUpdate   = "idc:admin:catalog:product:update"
	permissionCatalogProductQuery    = "idc:admin:catalog:product:query"
	permissionCatalogBillingWrite    = "idc:admin:catalog:billing:write"
	permissionCatalogPriceWrite      = "idc:admin:catalog:price:write"
	permissionCatalogTaxWrite        = "idc:admin:catalog:tax:write"
	permissionCatalogCostWrite       = "idc:admin:catalog:cost:write"
)

// CategoryCreateInput creates one active merchant-defined category.
type CategoryCreateInput struct {
	ParentCategoryNo string
	NameZhCN         string
	NameEnUS         string
	SortOrder        int
}

// CategoryUpdateInput updates one active category with optimistic concurrency.
type CategoryUpdateInput struct {
	CategoryNo       string
	ParentCategoryNo string
	NameZhCN         string
	NameEnUS         string
	SortOrder        int
	Version          int64
}

// CategoryStateInput changes one category deletion state with optimistic concurrency.
type CategoryStateInput struct {
	CategoryNo string
	Version    int64
}

// Category is the stable projection returned after a committed category write.
type Category struct {
	CategoryNo       string
	ParentCategoryNo string
	NameZhCN         string
	NameEnUS         string
	SortOrder        int
	Status           string
	Version          int64
	Deleted          bool
}

// ProductCreateInput creates a product and its empty manual draft version.
type ProductCreateInput struct {
	DirectoryNo string
	CategoryNo  string
	PublicSlug  string
	NameZhCN    string
	NameEnUS    string
	SummaryZhCN string
	SummaryEnUS string
	SortOrder   int

	InventoryTrackingEnabled bool
	SellableQuantity         int64
}

// ProductDraft is the committed product and initial blank-version projection.
type ProductDraft struct {
	ProductNo     string
	VersionNo     string
	VersionNumber int
	CategoryNo    string
	Status        string
	VersionStatus string
	Source        string
}

// ManagementAuthorizer fails closed before management persistence is accessed.
type ManagementAuthorizer interface {
	AuthorizeProductList(ctx context.Context) (actorUserID int, err error)
	AuthorizeProductQuery(ctx context.Context, productNo string) (actorUserID int, err error)
	AuthorizeCategoryList(ctx context.Context) (actorUserID int, err error)
	AuthorizeCategoryCreate(ctx context.Context) (actorUserID int, err error)
	AuthorizeCategoryUpdate(ctx context.Context, categoryNo string) (actorUserID int, err error)
	AuthorizeCategoryDelete(ctx context.Context, categoryNo string) (actorUserID int, err error)
	AuthorizeCategoryRestore(ctx context.Context, categoryNo string) (actorUserID int, err error)
	AuthorizeProductCreate(ctx context.Context) (actorUserID int, err error)
	// AuthorizeProductDelete checks deletion permission and the global merchant data scope.
	AuthorizeProductDelete(ctx context.Context, productNo string) (actorUserID int, err error)
	// AuthorizeProductUpdate checks the exact metadata permission and global merchant scope.
	AuthorizeProductUpdate(ctx context.Context, productNo string) (actorUserID int, err error)
	AuthorizeBillingWrite(ctx context.Context, versionNo string) (actorUserID int, err error)
	AuthorizePriceWrite(ctx context.Context, versionNo string) (actorUserID int, err error)
	AuthorizeTaxWrite(ctx context.Context, versionNo string) (actorUserID int, err error)
	AuthorizeCostWrite(ctx context.Context, versionNo string) (actorUserID int, err error)
}

// ConfigurationPriceTargetSource resolves merchant-selected local price tokens
// in the caller's transaction. It checks configuration permission and revision,
// and creates missing dimensions in a batch. Each source validates its accepted
// pricing modes; quantities require the merchant schema-aware source. Static
// options use saved data; dynamic candidates use the bounded local projection.
type ConfigurationPriceTargetSource interface {
	ResolvePriceTargets(context.Context, string, []string) (map[string]string, error)
}

// PriceBatchTargetKind prevents a missing option token from silently targeting
// the base price. An omitted kind retains the configuration-only contract.
type PriceBatchTargetKind string

const (
	PriceBatchTargetConfiguration PriceBatchTargetKind = "configuration"
	PriceBatchTargetBase          PriceBatchTargetKind = "base"
)

type ConfigurationPriceBatchRow struct {
	PricingMode     ConfigurationPriceMode
	TargetKind      PriceBatchTargetKind
	CandidateToken  string
	PreviousPriceNo string
	Schedule        PriceScheduleCreateInput
}

// ManagementService owns only general category, product, pricing, and tax writes.
type ManagementService interface {
	// SaveConfigurationPriceBatch saves at most 256 base or fixed-option schedules atomically. Base targets must be explicit; configuration targets retain configuration authorization. Both enforce price permission, draft locking and stale-price fences.
	SaveConfigurationPriceBatch(context.Context, string, []ConfigurationPriceBatchRow, ConfigurationPriceTargetSource) error
	// GetProductAddons returns at most 100 associated products after exact staff visibility checks.
	GetProductAddons(context.Context, string) ([]ProductAddon, error)
	// SetProductAddons replaces the bounded allowed offers with optimistic product concurrency and audit.
	SetProductAddons(context.Context, ProductAddonsInput) (int64, error)
	ImportWriter
	CreateCategory(ctx context.Context, input CategoryCreateInput) (*Category, error)
	UpdateCategory(ctx context.Context, input CategoryUpdateInput) (*Category, error)
	DeleteCategory(ctx context.Context, input CategoryStateInput) error
	RestoreCategory(ctx context.Context, input CategoryStateInput) (*Category, error)
	CreateProductDraft(ctx context.Context, input ProductCreateInput) (*ProductDraft, error)
	// DeleteProduct removes a product from new sales without deleting order or service facts; stale revisions fail atomically.
	DeleteProduct(ctx context.Context, input ProductDeleteInput) error
	// UpdateProductDescription updates bilingual sales copy and an optional sales category with optimistic concurrency and transactional audit.
	UpdateProductDescription(ctx context.Context, input ProductDescriptionUpdateInput) (int64, error)
	CreateBillingRule(ctx context.Context, input BillingRuleCreateInput) (*BillingRule, error)
	CreatePriceSchedule(ctx context.Context, input PriceScheduleCreateInput) (*PriceSchedule, error)
	// UpdatePriceSchedule replaces a draft schedule using its current row identity as a concurrency fence; permission and transactional audit are required.
	UpdatePriceSchedule(ctx context.Context, priceNo string, input PriceScheduleCreateInput) (*PriceSchedule, error)
	// DeletePriceSchedule removes all draft schedule tiers; published versions and stale row identities are rejected.
	DeletePriceSchedule(ctx context.Context, versionNo, priceNo string) error
	CreateTaxPolicy(ctx context.Context, input TaxPolicyCreateInput) (*TaxPolicy, error)
	ListProductVersionCosts(ctx context.Context, versionNo string) ([]ProductVersionCost, error)
	CreateProductVersionCost(ctx context.Context, input ProductVersionCostCreateInput) (*ProductVersionCost, error)
	UpdateProductVersionCost(ctx context.Context, input ProductVersionCostUpdateInput) (*ProductVersionCost, error)
	UpdateProductVersionInventory(ctx context.Context, input ProductVersionInventoryUpdateInput) (*ProductVersionInventory, error)
}

type managementServiceImpl struct {
	audit      idccap.BusinessAuditWriter
	authorizer ManagementAuthorizer

	random io.Reader
}

var _ ManagementService = (*managementServiceImpl)(nil)

// NewManagement creates the manual catalog management service.
func NewManagement(
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementAuthorizer,
) (ManagementService, error) {
	return newManagementServiceWithSources(auditWriter, authorizer, rand.Reader)
}

func newManagementServiceWithSources(
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementAuthorizer,

	random io.Reader,
) (*managementServiceImpl, error) {
	if nilManagementDependency(auditWriter) || nilManagementDependency(authorizer) || random == nil {
		return nil, gerror.New("catalog management requires audit, authorization, and random dependencies")
	}
	return &managementServiceImpl{
		audit: auditWriter, authorizer: authorizer,
		random: random,
	}, nil
}

func nilManagementDependency(value any) bool {
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
