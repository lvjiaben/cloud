// Paid periodic amounts come from immutable checkout facts, never from today's
// catalog renewal price. Installation charges and tax are excluded explicitly.
package serveroperation

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type nativePaidPeriod struct {
	QuoteItemNo, PhaseCode string
	NetAmountMinor         int64
	TotalAmountMinor       int64
	StartedAt              time.Time
}

func (s *NativePriceBuilder) paidPeriod(ctx context.Context, c NativePriceContext) (*nativePaidPeriod, error) {
	var row *nativePaidPeriod
	err := s.database.GetScan(ctx, &row, `SELECT COALESCE(price.source_quote_item_no,item.quote_item_no) AS quote_item_no,
 COALESCE(price.phase_code,'new_purchase') AS phase_code,COALESCE(price.net_amount_minor,0) AS net_amount_minor,COALESCE(price.total_amount_minor,0) AS total_amount_minor,
 COALESCE((SELECT (q.prepared_price#>>'{Context,StartedAt}')::TIMESTAMPTZ
 FROM plugin_idc_core_channel_service_quote q WHERE q.tenant_id=e.tenant_id AND q.quote_no=price.source_quote_no
 AND q.source_code='native' AND q.action_code='resize'),price.original_expires_at,e.started_at) AS started_at
 FROM plugin_idc_core_service_entitlement e
 JOIN plugin_idc_core_commerce_order_item item ON item.tenant_id=e.tenant_id AND item.order_item_no=e.order_item_no AND item.order_no=e.order_no
 LEFT JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=e.tenant_id AND price.price_snapshot_no=e.current_price_snapshot_no
 AND price.subject_type_code='entitlement' AND price.subject_no=e.entitlement_no AND price.customer_no=e.customer_no
 WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.currency_code=? AND e.version=?`, c.EntitlementNo, c.CustomerNo, c.CurrencyCode, c.EntitlementVersion)
	if err != nil {
		return nil, err
	}
	if row == nil || row.StartedAt.IsZero() {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	if row.PhaseCode != "resize_target" {
		var line catalog.PurchaseQuoteLine
		line, row.NetAmountMinor, err = s.periodicQuoteNet(ctx, row.QuoteItemNo)
		if err == nil {
			line.PriceIncludesTax = false
			line.Charges = []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: row.NetAmountMinor, PricedAmountMinor: row.NetAmountMinor}}
			amounts, amountErr := commerce.CalculatePricingAmounts(line, 0)
			err = amountErr
			row.TotalAmountMinor = amounts.TotalAmountMinor
		}
	}
	if err != nil || row.NetAmountMinor < 0 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return row, nil
}

func (s *NativePriceBuilder) periodicQuoteNet(ctx context.Context, itemNo string) (catalog.PurchaseQuoteLine, int64, error) {
	var line *catalog.PurchaseQuoteLine
	err := s.database.GetScan(ctx, &line, `SELECT i.product_no,i.version_no,i.billing_rule_no,qty.quantity,i.purchase_cycles,i.period_unit_code,i.period_count,
 i.tax_policy_no,i.price_includes_tax,i.tax_treatment_code,i.tax_rate_basis_points,i.tax_category_code,i.rounding_scope_code,i.rounding_mode_code,i.rounding_quantum_minor
 FROM plugin_idc_core_commerce_quote_item_snapshot i JOIN plugin_idc_core_commerce_quote_item_quantity_snapshot qty
 ON qty.tenant_id=i.tenant_id AND qty.quote_item_no=i.quote_item_no WHERE i.tenant_id=0 AND i.quote_item_no=?`, itemNo)
	if err != nil {
		return catalog.PurchaseQuoteLine{}, 0, err
	}
	if line == nil || line.Quantity != 1 {
		return catalog.PurchaseQuoteLine{}, 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	if err = s.database.GetScan(ctx, &line.Charges, `SELECT charge_kind_code AS kind_code,COALESCE(option_no,'') AS option_no,
 COALESCE(configuration_price_dimension_no,'') AS configuration_price_dimension_no,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest
 FROM plugin_idc_core_commerce_quote_charge_snapshot WHERE tenant_id=0 AND quote_item_no=? ORDER BY line_number LIMIT 5001`, itemNo); err != nil {
		return *line, 0, err
	}
	if len(line.Charges) == 0 || len(line.Charges) > 5000 {
		return *line, 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var adjustments []commerce.QuoteAdjustment
	if err = s.database.GetScan(ctx, &adjustments, `SELECT adjustment_type_code AS type_code,source_reference,source_version_digest,amount_minor,original_period_amount_minor,target_period_amount_minor
 FROM plugin_idc_core_commerce_quote_adjustment_snapshot WHERE tenant_id=0 AND quote_item_no=? ORDER BY application_order LIMIT 4097`, itemNo); err != nil {
		return *line, 0, err
	}
	if len(adjustments) > 4096 {
		return *line, 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	amount, err := commerce.CalculatePeriodicNetAmountWithAdjustments(*line, adjustments)
	return *line, amount, err
}
