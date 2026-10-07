// This file loads one bounded catalog workbench page with a single local projection query.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type productManagementRow struct {
	ProductManagementSummary
	InternalID int64 `orm:"internal_id"`
	SortOrder  int64 `orm:"sort_order"`
}

func searchProductManagementPage(
	ctx context.Context,
	input normalizedProductManagementSearch,
) (*ProductManagementPage, error) {
	boundarySort, boundaryID, hasCursor := int64(0), int64(0), 0
	if input.cursor != nil {
		boundarySort, boundaryID, hasCursor = input.cursor.SortOrder, input.cursor.ID, 1
	}
	args := []any{
		idccap.DefaultTenantID,
		input.namePrefix, input.likePrefix, input.likePrefix, input.likePrefix,
		input.categoryNo, input.categoryNo,
		input.status, input.status,
		input.source, input.source,
	}
	var (
		rows   = make([]productManagementRow, 0, input.limit+1)
		total  int
		offset int
	)
	if input.page > 0 {
		offset = (input.page - 1) * input.limit
	}
	// Both queries share the exact eligible-product projection. Only the page
	// query receives cursor/offset limits; pricing joins remain page-bounded.
	err := g.DB().Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := g.DB().Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		count, readErr := g.DB().GetValue(ctx, "SELECT COUNT(*) FROM ("+productManagementFilteredSQL+") eligible_products", args...)
		if readErr != nil {
			return readErr
		}
		total = count.Int()
		if offset >= total {
			return nil
		}
		pageArgs := append(append([]any(nil), args...), hasCursor, boundarySort, boundarySort, boundaryID, input.limit+1, offset)
		return g.DB().GetScan(ctx, &rows, productManagementPageSQL, pageArgs...)
	})
	if err != nil {
		return nil, gerror.Wrap(err, "load catalog product management page")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &ProductManagementPage{Products: make([]ProductManagementSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		page.Products = append(page.Products, row.ProductManagementSummary)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor, err := input.cursorCodec.encode(productManagementCursorBoundary{
			SortOrder: last.SortOrder, ID: last.InternalID,
		}, input)
		if err != nil {
			return nil, err
		}
		page.NextCursor = cursor
	}
	return page, nil
}

const productManagementFilteredSQL = `
    SELECT
        p.id AS internal_id, p.sort_order, p.product_no, p.category_no,
        c.name_zh_cn AS category_name_zh_cn, c.name_en_us AS category_name_en_us,
        v.version_no, v.version_number, p.status_code AS status,
        v.status_code AS version_status, v.source_code AS source,
        p.public_slug, p.name_zh_cn, p.name_en_us, p.summary_zh_cn, p.summary_en_us,
        v.published_at, p.created_at, p.updated_at
    FROM plugin_idc_core_catalog_product p
    INNER JOIN plugin_idc_core_catalog_category c
        ON c.category_no = p.category_no AND c.tenant_id = p.tenant_id
    INNER JOIN LATERAL (
        SELECT version_no, version_number, status_code, source_code, published_at
        FROM plugin_idc_core_catalog_product_version
        WHERE tenant_id = p.tenant_id AND product_no = p.product_no
        ORDER BY version_number DESC, id DESC
        LIMIT 1
    ) v ON TRUE
    WHERE p.tenant_id = ? AND p.deleted_at IS NULL AND c.deleted_at IS NULL
      AND p.product_no ~ '^prd_[0-9a-f]{32}$'
      AND c.category_no ~ '^ctg_[0-9a-f]{32}$'
      AND v.version_no ~ '^ver_[0-9a-f]{32}$'
      AND (? = '' OR LOWER(p.name_zh_cn) LIKE ? ESCAPE '\'
           OR LOWER(p.name_en_us) LIKE ? ESCAPE '\'
           OR p.public_slug LIKE ? ESCAPE '\')
	  AND (? = '' OR p.category_no = ?)
      AND (? = '' OR p.status_code = ?)
      AND (? = '' OR v.source_code = ?)`

const productManagementPageSQL = `
WITH eligible_products AS (` + productManagementFilteredSQL + `), page_products AS MATERIALIZED (
    SELECT * FROM eligible_products
    WHERE (? = 0 OR sort_order > ? OR (sort_order = ? AND internal_id > ?))
    ORDER BY sort_order ASC, internal_id ASC
    LIMIT ? OFFSET ?
), billing_stats AS (
    SELECT r.version_no, COUNT(*) AS total
    FROM plugin_idc_core_catalog_billing_rule r
    INNER JOIN page_products p ON p.version_no = r.version_no
    WHERE r.tenant_id = 0 AND r.status_code = 'active'
    GROUP BY r.version_no
), currency_stats AS (
    SELECT price.version_no, COUNT(DISTINCT price.currency_code) AS total
    FROM plugin_idc_core_catalog_price price
    INNER JOIN page_products p ON p.version_no = price.version_no
    WHERE price.tenant_id = 0 AND price.status_code = 'active'
    GROUP BY price.version_no
)
SELECT
    p.*,
    COALESCE(billing.total, 0) AS billing_rule_count,
    COALESCE(currencies.total, 0) AS currency_count
FROM page_products p
LEFT JOIN billing_stats billing ON billing.version_no = p.version_no
LEFT JOIN currency_stats currencies ON currencies.version_no = p.version_no
ORDER BY p.sort_order ASC, p.internal_id ASC`
