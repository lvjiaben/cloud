// This file loads the anonymous catalog with one product projection and one category query.

package catalog

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type publicCatalogProductRow struct {
	PublicCatalogProduct
	InternalID     int64  `orm:"internal_id"`
	SortOrder      int64  `orm:"sort_order"`
	CurrenciesJSON string `orm:"currencies_json"`
}

type publicCatalogCurrencyRow struct {
	CurrencyCode string `orm:"currency_code"`
}

func searchPublicCatalogPage(
	ctx context.Context,
	database gdb.DB,
	input normalizedPublicCatalogSearch,
) (*PublicCatalogPage, error) {
	rows, err := queryPublicCatalogProducts(ctx, database, input, "")
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &PublicCatalogPage{
		Products:            make([]PublicCatalogProduct, 0, len(rows)),
		Categories:          make([]PublicCatalogCategory, 0),
		AvailableCurrencies: make([]string, 0),
	}
	for index := range rows {
		if err = decodePublicCatalogCurrencies(&rows[index]); err != nil {
			return nil, err
		}
		page.Products = append(page.Products, rows[index].PublicCatalogProduct)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = input.cursorCodec.encode(publicCatalogCursorBoundary{
			SortOrder: last.SortOrder, ID: last.InternalID,
		}, input)
		if err != nil {
			return nil, err
		}
	}
	if err = database.GetScan(ctx, &page.Categories, publicCatalogCategoriesSQL,
		idccap.DefaultTenantID, maximumPublicCatalogCategoryCount+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load public catalog categories")
	}
	if len(page.Categories) > maximumPublicCatalogCategoryCount {
		return nil, gerror.New("public catalog category bound exceeded")
	}
	currencyRows := make([]publicCatalogCurrencyRow, 0, 65)
	if err = database.GetScan(ctx, &currencyRows, publicCatalogCurrenciesSQL, 65); err != nil {
		return nil, gerror.Wrap(err, "load public catalog currency choices")
	}
	if len(currencyRows) > 64 {
		return nil, gerror.New("public catalog currency choice bound exceeded")
	}
	for _, row := range currencyRows {
		page.AvailableCurrencies = append(page.AvailableCurrencies, row.CurrencyCode)
	}
	return page, nil
}

func loadPublicCatalogProduct(
	ctx context.Context,
	database gdb.DB,
	slug string,
) (*PublicCatalogProduct, error) {
	input := normalizedPublicCatalogSearch{limit: 1}
	rows, err := queryPublicCatalogProducts(ctx, database, input, slug)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	if err = decodePublicCatalogCurrencies(&rows[0]); err != nil {
		return nil, err
	}
	return &rows[0].PublicCatalogProduct, nil
}

func queryPublicCatalogProducts(
	ctx context.Context,
	database gdb.DB,
	input normalizedPublicCatalogSearch,
	slug string,
) ([]publicCatalogProductRow, error) {
	boundarySort, boundaryID, hasCursor := int64(0), int64(0), 0
	if input.cursor != nil {
		boundarySort, boundaryID, hasCursor = input.cursor.SortOrder, input.cursor.ID, 1
	}
	limit := input.limit + 1
	if slug != "" {
		limit = 1
	}
	rows := make([]publicCatalogProductRow, 0, limit)
	err := database.GetScan(ctx, &rows, publicCatalogProductsSQL,
		idccap.DefaultTenantID,
		slug, slug,
		input.namePrefix, input.likePrefix, input.likePrefix,
		input.categoryNo, input.categoryNo,
		input.currency, input.currency,
		hasCursor, boundarySort, boundarySort, boundaryID,
		limit,
		0,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "load public catalog products")
	}
	return rows, nil
}

func decodePublicCatalogCurrencies(row *publicCatalogProductRow) error {
	if row == nil {
		return gerror.New("public catalog row is nil")
	}
	if err := json.Unmarshal([]byte(row.CurrenciesJSON), &row.Currencies); err != nil {
		return gerror.Wrap(err, "decode public catalog currencies")
	}
	return nil
}

const publicCatalogCategoriesSQL = `
SELECT category_no, name_zh_cn, name_en_us
FROM plugin_idc_core_catalog_category
WHERE tenant_id = ? AND deleted_at IS NULL AND status_code = 'active'
ORDER BY sort_order ASC, id ASC
LIMIT ?`

const publicCatalogCurrenciesSQL = `
SELECT DISTINCT complete.currency_code
FROM (
    SELECT version.tenant_id, version.version_no, price.currency_code
    FROM plugin_idc_core_catalog_product product
    INNER JOIN plugin_idc_core_catalog_product_version version
        ON version.tenant_id = product.tenant_id AND version.product_no = product.product_no
       AND version.version_no = product.current_version_no AND version.status_code = 'published'
    INNER JOIN plugin_idc_core_catalog_price price
        ON price.tenant_id = product.tenant_id AND price.version_no = version.version_no
       AND price.option_no IS NULL AND price.configuration_price_dimension_no IS NULL AND price.status_code = 'active'
    INNER JOIN plugin_idc_core_merchant_currency currency
        ON currency.tenant_id = price.tenant_id AND currency.currency_code = price.currency_code
       AND currency.enabled = TRUE AND currency.deleted = FALSE
    INNER JOIN plugin_idc_core_catalog_billing_rule rule
        ON rule.tenant_id = price.tenant_id AND rule.version_no = price.version_no
       AND rule.billing_rule_no = price.billing_rule_no AND rule.status_code = 'active'
    WHERE product.tenant_id = 0 AND product.deleted_at IS NULL AND product.status_code = 'active'
    GROUP BY version.tenant_id, version.version_no, price.currency_code
    HAVING COUNT(DISTINCT price.billing_rule_no) = (
        SELECT COUNT(*)
        FROM plugin_idc_core_catalog_billing_rule all_rules
        WHERE all_rules.tenant_id = version.tenant_id
          AND all_rules.version_no = version.version_no
          AND all_rules.status_code = 'active'
    )
) complete
ORDER BY complete.currency_code ASC
LIMIT ?`

// publicCatalogProductsFilteredSQL is shared by cursor and numbered pages so
// unpublished, disabled and incompletely priced products have identical scope.
const publicCatalogProductsFilteredSQL = `
    SELECT
        p.id AS internal_id, p.sort_order, p.public_slug,
        p.category_no, c.name_zh_cn AS category_name_zh_cn,
        c.name_en_us AS category_name_en_us,
        p.name_zh_cn, p.name_en_us, p.summary_zh_cn, p.summary_en_us,
        v.version_no,
        inventory.tracking_enabled,
        inventory.sellable_quantity,
        COALESCE(reservation.reserved_quantity, 0) AS reserved_quantity
    FROM plugin_idc_core_catalog_product p
    INNER JOIN plugin_idc_core_catalog_category c
        ON c.tenant_id = p.tenant_id AND c.category_no = p.category_no
       AND c.deleted_at IS NULL AND c.status_code = 'active'
    INNER JOIN plugin_idc_core_catalog_product_version v
        ON v.tenant_id = p.tenant_id AND v.product_no = p.product_no
       AND v.version_no = p.current_version_no AND v.status_code = 'published'
    INNER JOIN plugin_idc_core_catalog_product_version_inventory inventory
        ON inventory.version_no = v.version_no
    LEFT JOIN (
        SELECT version_no, SUM(reserved_quantity)::BIGINT AS reserved_quantity
        FROM plugin_idc_core_commerce_inventory_reservation
        WHERE status_code IN ('held','committed')
        GROUP BY version_no
    ) reservation ON reservation.version_no = v.version_no
    WHERE p.tenant_id = ? AND p.deleted_at IS NULL AND p.status_code = 'active'
      AND (? = '' OR p.public_slug = ?)
      AND (? = '' OR LOWER(p.name_zh_cn) LIKE ? ESCAPE '\'
           OR LOWER(p.name_en_us) LIKE ? ESCAPE '\')
      AND (? = '' OR p.category_no = ?)
      AND EXISTS (
          SELECT 1
          FROM plugin_idc_core_catalog_price fp
          INNER JOIN plugin_idc_core_merchant_currency currency
              ON currency.tenant_id = fp.tenant_id AND currency.currency_code = fp.currency_code
             AND currency.enabled = TRUE AND currency.deleted = FALSE
          INNER JOIN plugin_idc_core_catalog_billing_rule fr
              ON fr.tenant_id = fp.tenant_id AND fr.version_no = fp.version_no
             AND fr.billing_rule_no = fp.billing_rule_no AND fr.status_code = 'active'
          WHERE fp.tenant_id = p.tenant_id AND fp.version_no = v.version_no
            AND fp.option_no IS NULL AND fp.configuration_price_dimension_no IS NULL AND fp.status_code = 'active'
            AND (? = '' OR fp.currency_code = ?)
          GROUP BY fp.currency_code
          HAVING COUNT(DISTINCT fp.billing_rule_no) = (
              SELECT COUNT(*)
              FROM plugin_idc_core_catalog_billing_rule all_rules
              WHERE all_rules.tenant_id = p.tenant_id
                AND all_rules.version_no = v.version_no
                AND all_rules.status_code = 'active'
          )
      )`

const publicCatalogProductsSQL = `
WITH page_products AS MATERIALIZED (` + publicCatalogProductsFilteredSQL + `
      AND (? = 0 OR p.sort_order > ? OR (p.sort_order = ? AND p.id > ?))
    ORDER BY p.sort_order ASC, p.id ASC
    LIMIT ?
    OFFSET ?
)
SELECT
    page.internal_id, page.sort_order, page.public_slug,
    page.category_no, page.category_name_zh_cn,
    page.category_name_en_us, page.name_zh_cn, page.name_en_us,
    page.summary_zh_cn, page.summary_en_us,
    currencies.currencies_json,
    CASE
        WHEN NOT page.tracking_enabled
          OR page.sellable_quantity > page.reserved_quantity
        THEN 'available'
        ELSE 'unavailable'
    END AS availability
FROM page_products page
INNER JOIN LATERAL (
    SELECT COALESCE(jsonb_agg(complete.currency_code ORDER BY complete.currency_code), '[]'::jsonb)::text
        AS currencies_json
    FROM (
        SELECT price.currency_code
        FROM plugin_idc_core_catalog_price price
        INNER JOIN plugin_idc_core_merchant_currency currency
            ON currency.tenant_id = price.tenant_id AND currency.currency_code = price.currency_code
           AND currency.enabled = TRUE AND currency.deleted = FALSE
        INNER JOIN plugin_idc_core_catalog_billing_rule rule
            ON rule.tenant_id = price.tenant_id AND rule.version_no = price.version_no
           AND rule.billing_rule_no = price.billing_rule_no AND rule.status_code = 'active'
        WHERE price.tenant_id = 0 AND price.version_no = page.version_no
          AND price.option_no IS NULL AND price.configuration_price_dimension_no IS NULL AND price.status_code = 'active'
        GROUP BY price.currency_code
        HAVING COUNT(DISTINCT price.billing_rule_no) = (
            SELECT COUNT(*)
            FROM plugin_idc_core_catalog_billing_rule all_rules
            WHERE all_rules.tenant_id = 0 AND all_rules.version_no = page.version_no
              AND all_rules.status_code = 'active'
        )
        ORDER BY price.currency_code
        LIMIT 64
    ) complete
) currencies ON currencies.currencies_json <> '[]'
ORDER BY page.sort_order ASC, page.internal_id ASC`
