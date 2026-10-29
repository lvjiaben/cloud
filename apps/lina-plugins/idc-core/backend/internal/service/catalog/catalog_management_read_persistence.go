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
	InternalID           int64 `orm:"internal_id"`
	BasePriceCurrency    string
	BasePriceAmount      string
	BasePriceSetupFee    string
	BasePricePeriodUnit  string
	BasePricePeriodCount int
	BasePriceChargeType  string
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
		if row.BasePriceCurrency != "" {
			row.BasePrice = &ProductManagementBasePrice{CurrencyCode: row.BasePriceCurrency, AmountMinor: row.BasePriceAmount, SetupFeeMinor: row.BasePriceSetupFee, PeriodUnit: row.BasePricePeriodUnit, PeriodCount: row.BasePricePeriodCount, ChargeType: row.BasePriceChargeType}
		}
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
), reservation_stats AS (
    SELECT sibling.product_no, SUM(reservation.reserved_quantity)::BIGINT AS reserved_quantity
    FROM page_products p
    JOIN plugin_idc_core_catalog_product_version sibling ON sibling.tenant_id=0 AND sibling.product_no=p.product_no
    JOIN plugin_idc_core_commerce_inventory_reservation reservation ON reservation.tenant_id=sibling.tenant_id AND reservation.version_no=sibling.version_no
    WHERE reservation.status_code IN ('held','committed')
    GROUP BY sibling.product_no
), delivery_stats AS (
    SELECT intent.product_no,
        SUM(CASE WHEN intent.granularity_code='per_quantity_unit' THEN 1 ELSE intent.quantity END)::BIGINT AS total_quantity,
        COALESCE(SUM(CASE WHEN intent.granularity_code='per_quantity_unit' THEN 1 ELSE intent.quantity END)
            FILTER (WHERE entitlement.entitlement_no IS NULL OR entitlement.status_code IN ('active','suspended','expired')),0)::BIGINT AS provisioned_quantity
    FROM page_products p
    JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=0 AND intent.product_no=p.product_no AND intent.status_code='completed'
    LEFT JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.tenant_id=intent.tenant_id AND entitlement.source_intent_no=intent.intent_no
    GROUP BY intent.product_no
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
), base_prices AS (
    SELECT price.version_no, price.currency_code, price.amount_minor, price.setup_fee_minor,
           r.period_unit_code, r.period_count, r.charge_type_code,
           ROW_NUMBER() OVER (PARTITION BY price.version_no ORDER BY
               price.currency_code, CASE r.period_unit_code
                   WHEN 'once' THEN 0 WHEN 'hour' THEN 1 WHEN 'day' THEN 2
                   WHEN 'month' THEN 3 WHEN 'year' THEN 4 ELSE 5 END,
               r.period_count, price.id) AS position
    FROM plugin_idc_core_catalog_price price
    INNER JOIN page_products p ON p.version_no=price.version_no
    INNER JOIN plugin_idc_core_catalog_billing_rule r
        ON r.billing_rule_no=price.billing_rule_no AND r.version_no=price.version_no AND r.tenant_id=price.tenant_id
    WHERE price.tenant_id=0 AND price.status_code='active' AND r.status_code='active'
      AND r.price_phase_code IN ('new_purchase','both')
      AND price.option_no IS NULL AND price.configuration_price_dimension_no IS NULL
      AND price.minimum_quantity <= 1 AND (price.maximum_quantity IS NULL OR price.maximum_quantity >= 1)
)
SELECT
    p.*,
    CASE WHEN inventory.tracking_enabled THEN GREATEST(inventory.sellable_quantity-COALESCE(reserved.reserved_quantity,0),0)::BIGINT ELSE NULL END AS available_quantity,
    COALESCE(delivery.provisioned_quantity,0) AS provisioned_quantity,
    COALESCE(delivery.total_quantity,0) AS total_quantity,
    base.currency_code AS base_price_currency,
    base.amount_minor::TEXT AS base_price_amount,
    base.setup_fee_minor::TEXT AS base_price_setup_fee,
    base.period_unit_code AS base_price_period_unit,
    base.period_count AS base_price_period_count,
    base.charge_type_code AS base_price_charge_type,
    COALESCE(billing.total, 0) AS billing_rule_count,
    COALESCE(currencies.total, 0) AS currency_count
FROM page_products p
LEFT JOIN plugin_idc_core_catalog_product_version_inventory inventory ON inventory.version_no=p.version_no
LEFT JOIN reservation_stats reserved ON reserved.product_no=p.product_no
LEFT JOIN delivery_stats delivery ON delivery.product_no=p.product_no
LEFT JOIN base_prices base ON base.version_no=p.version_no AND base.position=1
LEFT JOIN billing_stats billing ON billing.version_no = p.version_no
LEFT JOIN currency_stats currencies ON currencies.version_no = p.version_no
ORDER BY p.sort_order ASC, p.internal_id ASC`
