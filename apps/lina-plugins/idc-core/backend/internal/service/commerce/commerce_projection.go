// This file projects an already-owned cart and its current immutable quote with
// bounded set queries; it never resolves a plugin per displayed item.

package commerce

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

type latestCartQuoteRow struct {
	QuoteNo                   string    `orm:"quote_no"`
	CartNo                    string    `orm:"cart_no"`
	CurrencyCode              string    `orm:"currency_code"`
	CartVersion               int64     `orm:"cart_version"`
	QuoteItemCount            int       `orm:"quote_item_count"`
	QuoteSubtotalMinor        int64     `orm:"quote_subtotal_minor"`
	QuoteDiscountMinor        int64     `orm:"quote_discount_minor"`
	QuoteNetAmountMinor       int64     `orm:"quote_net_amount_minor"`
	QuoteTaxAmountMinor       int64     `orm:"quote_tax_amount_minor"`
	QuoteTotalAmountMinor     int64     `orm:"quote_total_amount_minor"`
	QuoteIssuedAt             time.Time `orm:"quote_issued_at"`
	QuoteExpiresAt            time.Time `orm:"quote_expires_at"`
	QuoteItemNo               string    `orm:"quote_item_no"`
	CartItemNo                string    `orm:"cart_item_no"`
	ProductNameZhCN           string    `orm:"product_name_zh_cn"`
	ProductNameEnUS           string    `orm:"product_name_en_us"`
	PublicSlug                string    `orm:"public_slug"`
	ConfigurationRequired     bool      `orm:"configuration_required"`
	ConfigurationFrozen       bool      `orm:"configuration_frozen"`
	ConfigurationRevision     string    `orm:"configuration_revision"`
	Quantity                  int       `orm:"quantity"`
	PurchaseCycles            int       `orm:"purchase_cycles"`
	FulfillmentModeCode       string    `orm:"fulfillment_mode_code"`
	RiskDecisionCode          string    `orm:"risk_decision_code"`
	InventoryDecisionCode     string    `orm:"inventory_decision_code"`
	SetupSubtotalMinor        int64     `orm:"setup_subtotal_minor"`
	PeriodSubtotalMinor       int64     `orm:"period_subtotal_minor"`
	SubtotalMinor             int64     `orm:"subtotal_minor"`
	DiscountMinor             int64     `orm:"discount_minor"`
	NetAmountMinor            int64     `orm:"net_amount_minor"`
	TaxAmountMinor            int64     `orm:"tax_amount_minor"`
	TotalAmountMinor          int64     `orm:"total_amount_minor"`
	AdjustmentOrder           int       `orm:"adjustment_order"`
	AdjustmentTypeCode        string    `orm:"adjustment_type_code"`
	AdjustmentSourceReference string    `orm:"adjustment_source_reference"`
	AdjustmentAmountMinor     int64     `orm:"adjustment_amount_minor"`
}

func (s *serviceImpl) projectGuestCart(ctx context.Context, digest string) (*Cart, error) {
	row, err := loadGuestCart(ctx, s.database, digest)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.projectCart(ctx, row)
}

func (s *serviceImpl) projectAccountCartByNumber(
	ctx context.Context,
	customerNo string,
	cartNo string,
) (*Cart, error) {
	row, err := loadCustomerCartByNumber(ctx, s.database, customerNo, cartNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.projectCart(ctx, row)
}

func (s *serviceImpl) projectCart(ctx context.Context, row *entity.CommerceCart) (*Cart, error) {
	items := make([]entity.CommerceCartItem, 0)
	itemColumns := dao.CommerceCartItem.Columns()
	if err := s.database.Model(dao.CommerceCartItem.Table()).Ctx(ctx).
		Where(do.CommerceCartItem{TenantId: idccap.DefaultTenantID, CartNo: row.CartNo}).
		WhereNot(itemColumns.StatusCode, "removed").
		OrderAsc(itemColumns.SortOrder).OrderAsc(itemColumns.Id).
		Limit(maximumCartItems + 1).Scan(&items); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(items) > maximumCartItems {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(items) == 0 {
		return &Cart{CartNo: row.CartNo, CurrencyCode: row.CurrencyCode, Version: row.Version, Items: []CartItem{}}, nil
	}
	type quantityRow struct {
		CartItemNo string `orm:"cart_item_no"`
		Quantity   int    `orm:"quantity"`
	}
	quantityRows := make([]quantityRow, 0, len(items))
	if err := s.database.GetScan(ctx, &quantityRows, `SELECT quantity.cart_item_no,quantity.quantity
FROM plugin_idc_core_commerce_cart_item_quantity quantity
JOIN plugin_idc_core_commerce_cart_item item ON item.tenant_id=quantity.tenant_id AND item.cart_item_no=quantity.cart_item_no
WHERE item.tenant_id=0 AND item.cart_no=? AND item.status_code<>'removed' ORDER BY item.sort_order,item.id LIMIT ?`, row.CartNo, maximumCartItems+1); err != nil || len(quantityRows) != len(items) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	quantityByItem := make(map[string]int, len(quantityRows))
	for _, q := range quantityRows {
		if q.Quantity < 1 {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		quantityByItem[q.CartItemNo] = q.Quantity
	}
	references := make([]catalog.PurchaseConfigurationReference, 0, len(items))
	for _, item := range items {
		references = append(references, catalog.PurchaseConfigurationReference{
			ReferenceNo: item.CartItemNo, ProductNo: item.ProductNo,
			VersionNo: item.VersionNo, BillingRuleNo: item.BillingRuleNo,
		})
	}
	states, err := s.catalog.RevalidatePurchaseConfigurations(ctx, references)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(states) != len(items) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	stateByReference := make(map[string]catalog.PurchaseConfigurationState, len(states))
	for _, state := range states {
		if _, duplicate := stateByReference[state.ReferenceNo]; duplicate {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		stateByReference[state.ReferenceNo] = state
	}
	result := &Cart{
		CartNo: row.CartNo, CurrencyCode: row.CurrencyCode,
		Version: row.Version, Items: make([]CartItem, 0, len(items)),
	}
	for _, item := range items {
		state, exists := stateByReference[item.CartItemNo]
		if !exists {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		issue := state.IssueCode
		status, repair := cartItemRepair(issue)
		if issue == "" && item.StatusCode != CartItemStatusActive {
			status, issue = item.StatusCode, item.IssueCode
			_, repair = cartItemRepair(issue)
		}
		projection := CartItem{
			CartItemNo: item.CartItemNo, ConfigurationReference: item.ConfigurationSnapshotNo, ProductNameZhCN: state.ProductNameZhCN,
			ProductNameEnUS: state.ProductNameEnUS, PublicSlug: state.PublicSlug,
			ConfigurationRequired: state.ConfigurationRequired, Quantity: quantityByItem[item.CartItemNo],
			PurchaseCycles: item.PurchaseCycles, StatusCode: status,
			IssueCode: issue, RepairAction: repair, Version: item.Version,
		}
		result.Items = append(result.Items, projection)
	}
	// Only the account DTO publishes these details; anonymous readers do not pay
	// for a display projection which their contract cannot consume.
	if row.CustomerNo != nil && *row.CustomerNo != "" {
		if err = s.projectCartConfiguration(ctx, result); err != nil {
			return nil, err
		}
	}
	latestQuote, err := s.loadLatestCartQuote(ctx, row)
	if err != nil {
		return nil, err
	}
	result.LatestQuote = latestQuote
	if latestQuote != nil {
		stateByCartItem := make(map[string]QuoteItem, len(latestQuote.Items))
		for _, quoteItem := range latestQuote.Items {
			if _, duplicate := stateByCartItem[quoteItem.CartItemNo]; duplicate {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			stateByCartItem[quoteItem.CartItemNo] = quoteItem
		}
		if len(stateByCartItem) != len(result.Items) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		for index := range result.Items {
			quoteItem, exists := stateByCartItem[result.Items[index].CartItemNo]
			if !exists {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			result.Items[index].ConfigurationFrozen = quoteItem.ConfigurationFrozen
			result.Items[index].ConfigurationRevision = quoteItem.ConfigurationRevision
		}
	}
	return result, nil
}

func (s *serviceImpl) loadLatestCartQuote(ctx context.Context, cart *entity.CommerceCart) (*Quote, error) {
	if cart == nil || cart.LatestQuoteNo == nil || *cart.LatestQuoteNo == "" {
		return nil, nil
	}
	rows := make([]latestCartQuoteRow, 0)
	err := s.database.GetScan(ctx, &rows, `SELECT
q.quote_no,q.cart_no,q.currency_code,cart.version AS cart_version,q.item_count AS quote_item_count,
q.subtotal_minor AS quote_subtotal_minor,q.discount_minor AS quote_discount_minor,
q.net_amount_minor AS quote_net_amount_minor,q.tax_amount_minor AS quote_tax_amount_minor,
q.total_amount_minor AS quote_total_amount_minor,q.issued_at AS quote_issued_at,q.expires_at AS quote_expires_at,
item.quote_item_no,item.cart_item_no,item.product_name_zh_cn,item.product_name_en_us,product.public_slug,
(configuration.version_no IS NOT NULL) AS configuration_required,
(frozen.snapshot_no IS NOT NULL) AS configuration_frozen,
COALESCE(frozen.configuration_revision,'') AS configuration_revision,
quantity.quantity,item.purchase_cycles,item.fulfillment_mode_code,item.risk_decision_code,item.inventory_decision_code,
item.setup_subtotal_minor,item.period_subtotal_minor,item.subtotal_minor,item.discount_minor,item.net_amount_minor,
item.tax_amount_minor,item.total_amount_minor,
COALESCE(adjustment.application_order,0) AS adjustment_order,
COALESCE(adjustment.adjustment_type_code,'') AS adjustment_type_code,
COALESCE(adjustment.source_reference,'') AS adjustment_source_reference,
COALESCE(adjustment.amount_minor,0) AS adjustment_amount_minor
FROM plugin_idc_core_commerce_cart cart
JOIN plugin_idc_core_commerce_quote_snapshot q
  ON q.tenant_id=cart.tenant_id AND q.cart_no=cart.cart_no AND q.quote_no=cart.latest_quote_no
JOIN plugin_idc_core_commerce_quote_item_snapshot item
  ON item.tenant_id=q.tenant_id AND item.quote_no=q.quote_no
JOIN plugin_idc_core_commerce_quote_item_quantity_snapshot quantity
  ON quantity.tenant_id=item.tenant_id AND quantity.quote_item_no=item.quote_item_no
JOIN plugin_idc_core_catalog_product product
  ON product.tenant_id=item.tenant_id AND product.product_no=item.product_no
LEFT JOIN plugin_idc_core_catalog_product_version_configuration configuration
  ON configuration.tenant_id=item.tenant_id AND configuration.version_no=item.version_no
LEFT JOIN plugin_idc_core_commerce_structured_configuration_snapshot frozen
  ON frozen.tenant_id=item.tenant_id AND frozen.quote_item_no=item.quote_item_no
LEFT JOIN plugin_idc_core_commerce_quote_adjustment_snapshot adjustment
  ON adjustment.tenant_id=item.tenant_id AND adjustment.quote_item_no=item.quote_item_no
WHERE cart.tenant_id=0 AND cart.cart_no=? AND cart.latest_quote_no=?
  AND cart.status_code='active' AND q.expires_at>CURRENT_TIMESTAMP
ORDER BY item.line_number,item.id,adjustment.application_order,adjustment.id
LIMIT ?`, cart.CartNo, *cart.LatestQuoteNo, maximumCartItems+maximumQuoteAdjustments+1)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) > maximumCartItems+maximumQuoteAdjustments {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	first := rows[0]
	quote := &Quote{
		QuoteNo: first.QuoteNo, CartNo: first.CartNo, CurrencyCode: first.CurrencyCode,
		CartVersion: first.CartVersion, SubtotalMinor: first.QuoteSubtotalMinor,
		DiscountMinor: first.QuoteDiscountMinor, NetAmountMinor: first.QuoteNetAmountMinor,
		TaxAmountMinor: first.QuoteTaxAmountMinor, TotalAmountMinor: first.QuoteTotalAmountMinor,
		IssuedAt: first.QuoteIssuedAt, ExpiresAt: first.QuoteExpiresAt,
		Items: make([]QuoteItem, 0, first.QuoteItemCount),
	}
	itemIndexes := make(map[string]int, first.QuoteItemCount)
	adjustmentCount := 0
	for _, row := range rows {
		if row.QuoteNo != quote.QuoteNo || row.CartNo != quote.CartNo || row.CurrencyCode != quote.CurrencyCode ||
			row.CartVersion != quote.CartVersion || row.QuoteItemCount != first.QuoteItemCount ||
			!row.QuoteExpiresAt.Equal(quote.ExpiresAt) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		index, exists := itemIndexes[row.QuoteItemNo]
		if !exists {
			if len(quote.Items) >= maximumCartItems || row.CartItemNo == "" ||
				(row.ConfigurationFrozen && (!row.ConfigurationRequired || row.ConfigurationRevision == "")) {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			itemIndexes[row.QuoteItemNo] = len(quote.Items)
			quote.Items = append(quote.Items, QuoteItem{
				QuoteItemNo: row.QuoteItemNo, CartItemNo: row.CartItemNo,
				ProductNameZhCN: row.ProductNameZhCN, ProductNameEnUS: row.ProductNameEnUS,
				PublicSlug: row.PublicSlug, ConfigurationRequired: row.ConfigurationRequired,
				ConfigurationFrozen: row.ConfigurationFrozen, ConfigurationRevision: row.ConfigurationRevision,
				Quantity: row.Quantity, PurchaseCycles: row.PurchaseCycles,
				FulfillmentModeCode: row.FulfillmentModeCode, RiskDecisionCode: row.RiskDecisionCode,
				InventoryDecisionCode: row.InventoryDecisionCode, SetupSubtotalMinor: row.SetupSubtotalMinor,
				PeriodSubtotalMinor: row.PeriodSubtotalMinor, SubtotalMinor: row.SubtotalMinor,
				DiscountMinor: row.DiscountMinor, NetAmountMinor: row.NetAmountMinor,
				TaxAmountMinor: row.TaxAmountMinor, TotalAmountMinor: row.TotalAmountMinor,
				Adjustments: []QuoteAdjustmentView{},
			})
			index = len(quote.Items) - 1
		}
		if row.AdjustmentOrder > 0 {
			adjustmentCount++
			if adjustmentCount > maximumQuoteAdjustments || row.AdjustmentTypeCode == "" ||
				row.AdjustmentSourceReference == "" || row.AdjustmentAmountMinor <= 0 {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			quote.Items[index].Adjustments = append(quote.Items[index].Adjustments, QuoteAdjustmentView{
				TypeCode: row.AdjustmentTypeCode, SourceReference: row.AdjustmentSourceReference,
				AmountMinor: row.AdjustmentAmountMinor,
			})
		}
	}
	if len(quote.Items) != first.QuoteItemCount {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return quote, nil
}

func cartItemRepair(issue string) (string, string) {
	switch issue {
	case "":
		return CartItemStatusActive, ""
	case catalog.PurchaseIssueProductUnavailable:
		return CartItemStatusUnavailable, CartRepairRemove
	case catalog.PurchaseIssueVersionReplaced:
		return CartItemStatusNeedsReprice, CartRepairReplaceVersion
	case catalog.PurchaseIssueBillingUnavailable:
		return CartItemStatusNeedsReprice, CartRepairRequote
	default:
		return CartItemStatusUnavailable, CartRepairRemove
	}
}

func loadGuestCart(ctx context.Context, database gdb.DB, digest string) (*entity.CommerceCart, error) {
	var row *entity.CommerceCart
	if err := database.GetScan(ctx, &row, loadGuestCartSQL, digest); err != nil {
		return nil, gerror.Wrap(err, "load guest cart")
	}
	return row, nil
}

func loadCustomerCart(ctx context.Context, database gdb.DB, customerNo string) (*entity.CommerceCart, error) {
	var row *entity.CommerceCart
	if err := database.GetScan(ctx, &row, loadCustomerCartSQL, customerNo); err != nil {
		return nil, gerror.Wrap(err, "load customer cart")
	}
	return row, nil
}

func loadCustomerCartByNumber(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	cartNo string,
) (*entity.CommerceCart, error) {
	var row *entity.CommerceCart
	if err := database.GetScan(ctx, &row, loadCustomerCartByNumberSQL, customerNo, cartNo); err != nil {
		return nil, gerror.Wrap(err, "load customer cart by number")
	}
	return row, nil
}

const loadGuestCartSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND guest_owner_digest = ? AND status_code = 'active'
  AND expires_at > CURRENT_TIMESTAMP
LIMIT 1`

const loadCustomerCartSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active'
  AND expires_at > CURRENT_TIMESTAMP
LIMIT 1`

const loadCustomerCartByNumberSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND customer_no = ? AND cart_no = ? AND status_code = 'active'
  AND expires_at > CURRENT_TIMESTAMP
LIMIT 1`
