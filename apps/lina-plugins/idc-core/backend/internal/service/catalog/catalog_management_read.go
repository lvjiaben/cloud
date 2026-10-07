// This file defines the bounded staff-only catalog product workbench projection.

package catalog

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
)

const (
	defaultCatalogManagementPageSize      = 25
	maximumCatalogManagementPageSize      = 100
	maximumCatalogManagementResponseBytes = 1024 * 1024
	maximumCatalogCategoryCount           = 500
	maximumCatalogCategoryResponseBytes   = 512 * 1024
)

const permissionCatalogProductList = "idc:admin:catalog:product:list"

type ProductManagementSearchInput struct {
	NamePrefix string
	CategoryNo string
	Status     string
	Source     string
	Cursor     string
	Page       int
	Limit      int
}

type ProductManagementSummary struct {
	ProductNo        string
	CategoryNo       string
	CategoryNameZhCN string
	CategoryNameEnUS string
	VersionNo        string
	VersionNumber    int
	Status           string
	VersionStatus    string
	Source           string
	PublicSlug       string
	NameZhCN         string
	NameEnUS         string
	SummaryZhCN      string
	SummaryEnUS      string
	BillingRuleCount int64
	CurrencyCount    int64
	PublishedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProductPauseImpactPreview is a read-only projection used before a product is
// paused. Pausing blocks new sales only; existing paid services remain intact.
type ProductPauseImpactPreview struct {
	ProductNo                 string
	Status                    string
	NewSalesBlocked           bool
	ActiveCartItems           int64
	UnpaidOrders              int64
	PaidOrdersInProgress      int64
	ExistingServices          int64
	ExistingServicesProtected bool
}

type ProductManagementPage struct {
	Products   []ProductManagementSummary
	NextCursor string
	Total      int
}

// CategoryManagementSummary is the bounded selector projection used by staff writes.
type CategoryManagementSummary struct {
	CategoryNo       string
	ParentCategoryNo string
	NameZhCN         string
	NameEnUS         string
	SortOrder        int
	Status           string
	Version          int64
	Deleted          bool
}

type ProductManagementReader interface {
	SearchProducts(ctx context.Context, input ProductManagementSearchInput) (*ProductManagementPage, error)
	GetProductDetail(ctx context.Context, productNo string) (*ProductManagementDetail, error)
	ListCategories(ctx context.Context, deletionState string) ([]CategoryManagementSummary, error)
	PreviewProductPauseImpact(ctx context.Context, productNo string) (*ProductPauseImpactPreview, error)
}

type productManagementReaderImpl struct {
	authorizer ManagementAuthorizer
	cursor     *productManagementCursorCodec
}

var _ ProductManagementReader = (*productManagementReaderImpl)(nil)

func NewProductManagementReader(
	authorizer ManagementAuthorizer,
	cursorKey []byte,
) (ProductManagementReader, error) {
	if nilManagementDependency(authorizer) {
		return nil, gerror.New("catalog product management reader requires authorization")
	}
	cursor, err := newProductManagementCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &productManagementReaderImpl{authorizer: authorizer, cursor: cursor}, nil
}

func (s *productManagementReaderImpl) SearchProducts(
	ctx context.Context,
	input ProductManagementSearchInput,
) (*ProductManagementPage, error) {
	if _, err := s.authorizer.AuthorizeProductList(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizeProductManagementSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := searchProductManagementPage(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if err = validateProductManagementPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return page, nil
}

// PreviewProductPauseImpact returns bounded counts from the authoritative
// catalog, commerce, and service tables. It never changes product or service
// state and deliberately does not count terminated services as active impact.
func (s *productManagementReaderImpl) PreviewProductPauseImpact(ctx context.Context, productNo string) (*ProductPauseImpactPreview, error) {
	if _, err := s.authorizer.AuthorizeProductQuery(ctx, productNo); err != nil {
		return nil, err
	}
	if !existingCatalogReferencePattern.MatchString(productNo) {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	var preview ProductPauseImpactPreview
	err := dao.CatalogProduct.Ctx(ctx).Raw(`
SELECT p.product_no,
       p.status_code,
       (p.status_code = 'suspended') AS new_sales_blocked,
       COALESCE((SELECT COUNT(*) FROM plugin_idc_core_commerce_cart_item ci
                 WHERE ci.tenant_id = 0 AND ci.product_no = p.product_no
                   AND ci.status_code <> 'removed'), 0) AS active_cart_items,
       COALESCE((SELECT COUNT(DISTINCT o.order_no) FROM plugin_idc_core_commerce_order o
                 JOIN plugin_idc_core_commerce_order_item oi ON oi.order_no = o.order_no
                 WHERE o.tenant_id = 0 AND oi.product_no = p.product_no
                   AND o.status_code IN ('pending_payment', 'payment_processing')), 0) AS unpaid_orders,
       COALESCE((SELECT COUNT(DISTINCT o.order_no) FROM plugin_idc_core_commerce_order o
                 JOIN plugin_idc_core_commerce_order_item oi ON oi.order_no = o.order_no
                 WHERE o.tenant_id = 0 AND oi.product_no = p.product_no
                 AND o.status_code IN ('paid', 'fulfilling', 'partially_fulfilled')), 0) AS paid_orders_in_progress,
       COALESCE((SELECT COUNT(*) FROM plugin_idc_core_service_instance si
                 WHERE si.tenant_id = 0 AND si.product_no = p.product_no
                   AND si.entitlement_status_code NOT IN ('terminated', 'cancelled')), 0) AS existing_services,
       TRUE AS existing_services_protected
FROM plugin_idc_core_catalog_product p
WHERE p.tenant_id = 0 AND p.product_no = ?`, productNo).Scan(&preview)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if preview.ProductNo == "" {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return &preview, nil
}

type normalizedProductManagementSearch struct {
	namePrefix  string
	likePrefix  string
	categoryNo  string
	status      string
	source      string
	cursor      *productManagementCursorBoundary
	cursorCodec *productManagementCursorCodec
	limit       int
	page        int
}

func normalizeProductManagementSearch(
	input ProductManagementSearchInput,
	cursorCodec *productManagementCursorCodec,
) (normalizedProductManagementSearch, error) {
	namePrefix := strings.ToLower(strings.Join(strings.Fields(input.NamePrefix), " "))
	if !utf8.ValidString(namePrefix) || utf8.RuneCountInString(namePrefix) > 128 ||
		(input.CategoryNo != "" && !categoryNumberPattern.MatchString(input.CategoryNo)) ||
		!validProductManagementStatus(input.Status) ||
		!validProductManagementSource(input.Source) {
		return normalizedProductManagementSearch{}, bizerr.NewCode(CodeManagementInvalid)
	}
	if input.Limit == 0 {
		input.Limit = defaultCatalogManagementPageSize
	}
	if input.Limit < 1 || input.Limit > maximumCatalogManagementPageSize || input.Page < 0 ||
		(input.Page > 0 && (input.Cursor != "" || input.Page-1 > math.MaxInt/input.Limit)) {
		return normalizedProductManagementSearch{}, bizerr.NewCode(CodeManagementInvalid)
	}
	normalized := normalizedProductManagementSearch{
		namePrefix: namePrefix, likePrefix: escapeCatalogLikePrefix(namePrefix),
		categoryNo: input.CategoryNo, status: input.Status, source: input.Source,
		cursorCodec: cursorCodec, limit: input.Limit, page: input.Page,
	}
	if cursorCodec == nil {
		return normalizedProductManagementSearch{}, bizerr.NewCode(CodeManagementInvalid)
	}
	if input.Cursor != "" {
		cursor, err := cursorCodec.decode(input.Cursor, normalized)
		if err != nil {
			return normalizedProductManagementSearch{}, bizerr.NewCode(CodeManagementInvalid)
		}
		normalized.cursor = cursor
	}
	return normalized, nil
}

func validProductManagementStatus(value string) bool {
	switch value {
	case "", "draft", "active", "suspended", "retired":
		return true
	default:
		return false
	}
}

func validProductManagementSource(value string) bool {
	switch value {
	case "", "manual", "migration", "copy":
		return true
	default:
		return false
	}
}

func escapeCatalogLikePrefix(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value) + "%"
}

func validateProductManagementPage(page *ProductManagementPage) error {
	if page == nil || page.Products == nil || len(page.Products) > maximumCatalogManagementPageSize ||
		(page.NextCursor != "" && len(page.NextCursor) > maximumProductManagementCursorLength) {
		return gerror.New("catalog product management page is invalid")
	}
	for _, product := range page.Products {
		if !existingCatalogReferencePattern.MatchString(product.ProductNo) ||
			!existingCatalogReferencePattern.MatchString(product.VersionNo) || product.VersionNumber < 1 ||
			product.BillingRuleCount < 0 || product.CurrencyCount < 0 ||
			product.CreatedAt.IsZero() || product.UpdatedAt.IsZero() {
			return gerror.New("catalog product management projection is inconsistent")
		}
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		return gerror.Wrap(err, "measure catalog product management page")
	}
	if len(encoded) > maximumCatalogManagementResponseBytes {
		return gerror.New("catalog product management page exceeds response bound")
	}
	return nil
}
