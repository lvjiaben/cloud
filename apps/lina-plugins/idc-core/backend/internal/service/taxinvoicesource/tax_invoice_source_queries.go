// This file owns bounded database projections and invoice-row locking.
package taxinvoicesource

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// profileRow is the core-owned invoice-profile database projection.
type profileRow struct {
	ProfileNo            string `orm:"profile_no"`
	TitleTypeCode        string `orm:"title_type_code"`
	InvoiceTitle         string `orm:"invoice_title"`
	TaxRegistrationNo    string `orm:"tax_registration_no"`
	RegisteredAddress    string `orm:"registered_address"`
	RegisteredPhone      string `orm:"registered_phone"`
	BankName             string `orm:"bank_name"`
	BankAccountMasked    string `orm:"bank_account_masked"`
	BankAccountSecretRef string `orm:"bank_account_secret_ref"`
	IsDefault            bool   `orm:"is_default"`
	Version              int64  `orm:"version"`
}

// merchantRow is the installation-local seller database projection.
type merchantRow struct {
	MerchantNo  string `orm:"merchant_no"`
	LegalName   string `orm:"legal_name"`
	CountryCode string `orm:"country_code"`
	Version     int64  `orm:"version"`
}

// invoiceRow is the immutable paid billing-invoice database projection.
type invoiceRow struct {
	InvoiceNo           string     `orm:"invoice_no"`
	OrderNo             string     `orm:"order_no"`
	CurrencyCode        string     `orm:"currency_code"`
	PaidAmountMinor     int64      `orm:"paid_amount_minor"`
	RefundedAmountMinor int64      `orm:"refunded_amount_minor"`
	ContentDigest       string     `orm:"content_digest"`
	SettledAt           *time.Time `orm:"settled_at"`
	Version             int64      `orm:"version"`
}

// itemRow is a billing item joined to its authoritative tax facts and refunds.
type itemRow struct {
	InvoiceNo           string `orm:"invoice_no"`
	InvoiceItemNo       string `orm:"invoice_item_no"`
	SourceReference     string `orm:"source_reference"`
	DescriptionZhCN     string `orm:"description_zh_cn"`
	DescriptionEnUS     string `orm:"description_en_us"`
	NetAmountMinor      int64  `orm:"net_amount_minor"`
	TaxAmountMinor      int64  `orm:"tax_amount_minor"`
	TotalAmountMinor    int64  `orm:"total_amount_minor"`
	RefundedAmountMinor int64  `orm:"refunded_amount_minor"`
	TaxTreatmentCode    string `orm:"tax_treatment_code"`
	TaxRateBasisPoints  *int   `orm:"tax_rate_basis_points"`
	TaxCategoryCode     string `orm:"tax_category_code"`
	SourceContentDigest string `orm:"source_content_digest"`
}

// databaseNow reads the database clock used by the same source transaction.
func databaseNow(ctx context.Context, tx gdb.TX) (time.Time, error) {
	value, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, err
	}
	return value.Time().UTC(), nil
}

// loadProfiles lists active profiles for the authenticated customer.
func loadProfiles(ctx context.Context, tx gdb.TX, customerNo string) ([]profileRow, error) {
	var rows []profileRow
	err := tx.GetScan(&rows, `
SELECT profile_no, title_type_code, invoice_title, tax_registration_no,
       registered_address, registered_phone, bank_name, bank_account_masked,
       is_default, version
FROM plugin_idc_core_customer_invoice_profile
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active'
ORDER BY is_default DESC, id ASC
LIMIT ?`, customerNo, taxinvoicecap.MaximumProfiles+1)
	if err != nil {
		return nil, err
	}
	if len(rows) > taxinvoicecap.MaximumProfiles {
		return nil, gerror.New("customer invoice profile source exceeds the supported bound")
	}
	for index := range rows {
		if err = validateProfileSummaryRow(&rows[index]); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// loadSelectedProfile reads one exact active profile owned by the customer.
func loadSelectedProfile(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	profileNo string,
) (*profileRow, error) {
	var row *profileRow
	err := tx.GetScan(&row, `
SELECT profile_no, title_type_code, invoice_title, tax_registration_no,
       registered_address, registered_phone, bank_name, bank_account_masked,
       bank_account_secret_ref, is_default, version
FROM plugin_idc_core_customer_invoice_profile
WHERE tenant_id = 0 AND customer_no = ? AND profile_no = ? AND status_code = 'active'
LIMIT 1`, customerNo, profileNo)
	if err != nil {
		return nil, err
	}
	if row == nil || validateProfile(row) != nil {
		return nil, gerror.New("customer invoice profile is not accessible")
	}
	return row, nil
}

// loadMerchant reads the enabled installation-local seller identity.
func loadMerchant(ctx context.Context, tx gdb.TX) (*merchantRow, error) {
	var row *merchantRow
	err := tx.GetScan(&row, `
SELECT merchant_no, legal_name, country_code, version
FROM plugin_idc_core_merchant_profile
WHERE tenant_id = 0 AND enabled
LIMIT 1`)
	if err != nil {
		return nil, err
	}
	if row == nil || !businessNoPattern.MatchString(row.MerchantNo) ||
		strings.TrimSpace(row.LegalName) == "" || len(row.CountryCode) != 2 || row.Version <= 0 {
		return nil, gerror.New("installation merchant tax invoice source is unavailable")
	}
	return row, nil
}

// loadInvoiceOptions lists bounded paid invoices for the authenticated customer.
func loadInvoiceOptions(ctx context.Context, tx gdb.TX, customerNo string) ([]invoiceRow, error) {
	var rows []invoiceRow
	err := tx.GetScan(&rows, `
SELECT invoice_no, COALESCE(order_no, '') AS order_no, currency_code,
       paid_amount_minor, refunded_amount_minor, content_digest, settled_at, version
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND customer_no = ?
  AND status_code IN ('paid', 'partially_refunded')
  AND settled_at IS NOT NULL AND cancelled_at IS NULL
  AND paid_amount_minor > refunded_amount_minor
ORDER BY settled_at DESC, id DESC
LIMIT ?`, customerNo, taxinvoicecap.MaximumInvoices+1)
	if err != nil {
		return nil, err
	}
	if len(rows) > taxinvoicecap.MaximumInvoices {
		return nil, gerror.New("paid billing invoice source exceeds the supported bound")
	}
	return rows, nil
}

// lockSelectedInvoices locks a stable, current-customer invoice set.
func lockSelectedInvoices(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	invoiceNos []string,
) ([]invoiceRow, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(invoiceNos)), ",")
	arguments := make([]any, 0, len(invoiceNos)+1)
	arguments = append(arguments, customerNo)
	for _, invoiceNo := range invoiceNos {
		arguments = append(arguments, invoiceNo)
	}
	var rows []invoiceRow
	err := tx.GetScan(&rows, `
SELECT invoice_no, COALESCE(order_no, '') AS order_no, currency_code,
       paid_amount_minor, refunded_amount_minor, content_digest, settled_at, version
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND customer_no = ?
  AND invoice_no IN (`+placeholders+`)
  AND status_code IN ('paid', 'partially_refunded')
  AND settled_at IS NOT NULL AND cancelled_at IS NULL
ORDER BY invoice_no ASC
FOR UPDATE`, arguments...)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(invoiceNos) {
		return nil, gerror.New("selected paid billing invoice is not accessible")
	}
	return rows, nil
}

// loadSelectedItems loads every selected item, its source lineage, tax facts,
// and effective refund usage in one bounded query.
func loadSelectedItems(ctx context.Context, tx gdb.TX, invoiceNos []string) ([]itemRow, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(invoiceNos)), ",")
	arguments := make([]any, len(invoiceNos))
	for index := range invoiceNos {
		arguments[index] = invoiceNos[index]
	}
	queryArguments := make([]any, 0, len(arguments)*2+1)
	queryArguments = append(queryArguments, arguments...)
	queryArguments = append(queryArguments, arguments...)
	queryArguments = append(queryArguments, taxinvoicecap.MaximumItemsPerRequest+1)
	var rows []itemRow
	err := tx.GetScan(&rows, `
WITH RECURSIVE selected AS (
    SELECT item.id, item.invoice_item_no, item.invoice_no, item.order_item_no,
           item.item_type_code, item.source_reference, item.line_number,
           item.description_zh_cn, item.description_en_us,
           item.net_amount_minor, item.tax_amount_minor, item.total_amount_minor,
           item.content_digest
    FROM plugin_idc_core_billing_invoice_item AS item
    WHERE item.tenant_id = 0 AND item.invoice_no IN (`+placeholders+`)
), lineage AS (
    SELECT selected.invoice_item_no AS leaf_item_no,
           selected.invoice_item_no AS current_item_no,
           selected.invoice_no AS current_invoice_no,
           selected.order_item_no, selected.item_type_code,
           selected.source_reference, selected.content_digest, 0 AS depth
    FROM selected
    UNION ALL
    SELECT lineage.leaf_item_no, parent.invoice_item_no, parent.invoice_no,
           parent.order_item_no, parent.item_type_code,
           parent.source_reference, parent.content_digest, lineage.depth + 1
    FROM lineage
    JOIN plugin_idc_core_billing_invoice_item_derivation AS derivation
      ON derivation.tenant_id = 0
     AND derivation.destination_invoice_item_no = lineage.current_item_no
    JOIN plugin_idc_core_billing_invoice_item AS parent
      ON parent.tenant_id = 0
     AND parent.invoice_item_no = derivation.source_invoice_item_no
    WHERE lineage.depth < 16
), root AS (
    SELECT DISTINCT ON (leaf_item_no)
           leaf_item_no, current_item_no, current_invoice_no, order_item_no,
           item_type_code, source_reference, content_digest
    FROM lineage
    ORDER BY leaf_item_no, depth DESC
), refunds AS (
    SELECT refund.invoice_item_no,
           COALESCE(SUM(CASE refund.direction_code
               WHEN 'apply' THEN refund.amount_minor ELSE -refund.amount_minor END), 0) AS refunded_amount_minor
    FROM plugin_idc_core_billing_refund_allocation AS refund
    WHERE refund.tenant_id = 0 AND refund.invoice_no IN (`+placeholders+`)
    GROUP BY refund.invoice_item_no
)
SELECT selected.invoice_no, selected.invoice_item_no, selected.source_reference,
       selected.description_zh_cn, selected.description_en_us,
       selected.net_amount_minor, selected.tax_amount_minor,
       selected.total_amount_minor,
       COALESCE(refunds.refunded_amount_minor, 0) AS refunded_amount_minor,
       COALESCE(order_item.tax_treatment_code, price.tax_treatment_code, '') AS tax_treatment_code,
       COALESCE(order_item.tax_rate_basis_points, price.tax_rate_basis_points) AS tax_rate_basis_points,
       COALESCE(order_item.tax_category_code, price.tax_category_code, '') AS tax_category_code,
       root.content_digest AS source_content_digest
FROM selected
JOIN root ON root.leaf_item_no = selected.invoice_item_no
LEFT JOIN plugin_idc_core_commerce_order_item AS order_item
  ON order_item.tenant_id = 0 AND order_item.order_item_no = root.order_item_no
LEFT JOIN plugin_idc_core_business_operation AS operation
  ON operation.tenant_id = 0 AND operation.invoice_no = root.current_invoice_no
LEFT JOIN plugin_idc_core_service_price_snapshot AS price
  ON price.tenant_id = 0 AND price.price_snapshot_no = operation.price_snapshot_no
LEFT JOIN refunds ON refunds.invoice_item_no = selected.invoice_item_no
ORDER BY selected.invoice_no ASC, selected.line_number ASC, selected.id ASC
LIMIT ?`, queryArguments...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || len(rows) > taxinvoicecap.MaximumItemsPerRequest {
		return nil, gerror.New("selected billing invoice item source exceeds the supported bound")
	}
	for index := range rows {
		if err = validateItemRow(&rows[index]); err != nil {
			return nil, err
		}
	}
	return rows, nil
}
