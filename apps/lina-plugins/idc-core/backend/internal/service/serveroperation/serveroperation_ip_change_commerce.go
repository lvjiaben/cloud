package serveroperation

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (s *accessoryPricingImpl) persistIPChangeCommerceQuote(ctx context.Context,
	customerNo, _ string, requestDigest, quoteContentDigest string, service *ipChangePreviewRow,
	directory *serveroperationresourcecap.AccessoryProjection, quote *IPChangeQuote, now time.Time) error {
	value, err := s.loadIPChangeCommerceContext(ctx, service, directory, quote.AmountMinor)
	if err != nil {
		return err
	}
	quoteItemNo, _ := deterministicReference("iqi_", quote.QuoteNo)
	quoteChargeNo, _ := deterministicReference("iqc_", quote.QuoteNo)
	priceNo, _ := deterministicReference("ips_", quote.QuoteNo)
	componentNo, _ := deterministicReference("ipm_", quote.QuoteNo+":base")
	pricingDigest, err := digestValue(struct {
		PolicyNo string
		Version  int64
		Amount   int64
	}{quote.PolicyNo, quote.PolicyVersion, quote.AmountMinor})
	if err != nil {
		return err
	}
	eligibilityDigest, err := digestValue(struct {
		CustomerNo, ServiceNo, ReservationNo string
		CustomerVersion, ServiceVersion      int64
	}{customerNo, quote.ServiceNo, quote.ResourceReservationNo,
		service.CustomerSubjectVersion, service.Version})
	if err != nil {
		return err
	}
	itemContent, err := digestValue(struct {
		QuoteItemNo, QuoteNo, ServiceNo, ReservationNo string
		IPQuantity                                     int
		Amount                                         int64
	}{quoteItemNo, quote.QuoteNo, quote.ServiceNo, quote.ResourceReservationNo,
		quote.IPQuantity, quote.AmountMinor})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_snapshot
(tenant_id,quote_no,customer_no,customer_subject_version,purpose_code,source_code,currency_code,item_count,
subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,input_digest,
pricing_rule_digest,eligibility_digest,inventory_digest,content_digest,issued_at,expires_at,created_by)
VALUES (0,?,?,?,'ip_change','customer',?,1,?,0,?,?,?,?,?,?,?,?,?,?,0)`, quote.QuoteNo, customerNo,
		service.CustomerSubjectVersion, quote.CurrencyCode, value.SubtotalMinor, value.NetAmountMinor,
		value.TaxAmountMinor, quote.AmountMinor, requestDigest, pricingDigest, eligibilityDigest,
		value.CapabilityEvidenceDigest, quoteContentDigest, now, quote.ExpiresAt); err != nil {
		return gerror.Wrap(err, "insert IP-change commerce quote")
	}
	item := do.CommerceQuoteItemSnapshot{
		TenantId: 0, QuoteItemNo: quoteItemNo, QuoteNo: quote.QuoteNo, LineNumber: 1,
		CurrencyCode: quote.CurrencyCode, ProductNo: value.ProductNo, VersionNo: value.VersionNo,
		BillingRuleNo: value.BillingRuleNo, ConfigurationSnapshotNo: value.ConfigurationSnapshotNo,
		ProductNameZhCn: value.NameZhCN, ProductNameEnUs: value.NameEnUS, PurchaseCycles: 1,
		ChargeTypeCode: "one_time", PeriodUnitCode: "once", PeriodCount: 1,
		FulfillmentModeCode: "automatic", RiskDecisionCode: "approved",
		InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: value.CapabilityEvidenceDigest,
		TaxPolicyNo: accessoryTaxValue(value, "policy"), PriceIncludesTax: true,
		TaxTreatmentCode: accessoryTaxValue(value, "treatment"), TaxRateBasisPoints: value.TaxRateBasisPoints,
		TaxCategoryCode: accessoryTaxValue(value, "category"), RoundingScopeCode: accessoryTaxValue(value, "scope"),
		RoundingModeCode: accessoryTaxValue(value, "mode"), RoundingQuantumMinor: accessoryTaxQuantum(value),
		SetupSubtotalMinor: value.SubtotalMinor, PeriodSubtotalMinor: 0,
		SubtotalMinor: value.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: value.NetAmountMinor,
		TaxAmountMinor: value.TaxAmountMinor, TotalAmountMinor: quote.AmountMinor, ContentDigest: itemContent,
	}
	if _, err = s.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(item).Insert(); err != nil {
		return gerror.Wrap(err, "insert IP-change commerce quote item")
	}
	quantityDigest, err := digestValue(struct {
		QuoteItemNo string
		Quantity    int
	}{quoteItemNo, 1})
	if err != nil {
		return err
	}
	if _, err = s.database.Model("plugin_idc_core_commerce_quote_item_quantity_snapshot").Ctx(ctx).Data(map[string]any{
		"tenant_id": 0, "quote_item_no": quoteItemNo, "quantity": 1, "content_digest": quantityDigest,
	}).Insert(); err != nil {
		return gerror.Wrap(err, "insert IP-change commerce quote quantity")
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot
(tenant_id,quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,charge_phase_code,
quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest)
VALUES (0,?,?,?,1,'base','setup',1,?,0,?,?)`, quoteChargeNo, quoteItemNo,
		quote.CurrencyCode, quote.AmountMinor, quote.AmountMinor, pricingDigest); err != nil {
		return gerror.Wrap(err, "insert IP-change commerce quote charge")
	}
	if quote.AmountMinor == 0 {
		return nil
	}
	priceContent, err := digestValue(struct {
		PriceNo, QuoteNo, QuoteItemNo, ServiceNo, Evidence string
		Amount                                             int64
	}{priceNo, quote.QuoteNo, quoteItemNo, quote.ServiceNo,
		value.CapabilityEvidenceDigest, quote.AmountMinor})
	if err != nil {
		return err
	}
	componentCount := 1
	if value.TaxAmountMinor > 0 {
		componentCount++
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
(tenant_id,price_snapshot_no,service_no,customer_no,phase_code,source_quote_no,source_quote_item_no,
product_no,version_no,billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,
period_unit_code,period_count,renewal_mode_code,capability_evidence_no,capability_evidence_digest,
capability_evidence_observed_at,capability_evidence_expires_at,original_expires_at,projected_expires_at,
tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,
rounding_scope_code,rounding_mode_code,rounding_quantum_minor,component_count,subtotal_minor,
discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
VALUES (0,?,?,?,'ip_change',?,?,?,?,?,?,?,1,'once',1,'provider_confirmed',?,?,?, ?,NULL,NULL,
?,TRUE,?,?,?,?,?,?,?,?,0,?,?,?,?,?)`, priceNo, quote.ServiceNo, customerNo, quote.QuoteNo,
		quoteItemNo, value.ProductNo, value.VersionNo, value.BillingRuleNo,
		value.ConfigurationSnapshotNo, quote.CurrencyCode, value.CapabilityEvidenceNo,
		value.CapabilityEvidenceDigest, directory.ObservedAt, directory.ExpiresAt,
		value.TaxPolicyNo, value.TaxTreatmentCode, value.TaxRateBasisPoints,
		value.TaxCategoryCode, value.RoundingScopeCode, value.RoundingModeCode,
		value.RoundingQuantumMinor, componentCount, value.SubtotalMinor, value.NetAmountMinor,
		value.TaxAmountMinor, quote.AmountMinor, pricingDigest, priceContent); err != nil {
		return gerror.Wrap(err, "insert IP-change price snapshot")
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(tenant_id,component_snapshot_no,price_snapshot_no,service_no,line_number,component_type_code,
charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES (0,?,?,?,1,'base','setup',?,1,?,?,?,?)`, componentNo, priceNo, quote.ServiceNo,
		quote.PolicyNo, value.NetAmountMinor, value.NetAmountMinor, pricingDigest, itemContent); err != nil {
		return gerror.Wrap(err, "insert IP-change base price component")
	}
	if value.TaxAmountMinor > 0 {
		taxComponentNo, _ := deterministicReference("ipm_", quote.QuoteNo+":tax")
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(tenant_id,component_snapshot_no,price_snapshot_no,service_no,line_number,component_type_code,
charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES (0,?,?,?,2,'tax','total',?,1,?,?,?,?)`, taxComponentNo, priceNo, quote.ServiceNo,
			value.TaxPolicyNo, value.TaxAmountMinor, value.TaxAmountMinor, pricingDigest, quoteContentDigest); err != nil {
			return gerror.Wrap(err, "insert IP-change tax price component")
		}
	}
	return nil
}

func (s *accessoryPricingImpl) loadIPChangeCommerceContext(ctx context.Context,
	service *ipChangePreviewRow, directory *serveroperationresourcecap.AccessoryProjection,
	total int64) (*accessoryCommerceContext, error) {
	if service.CurrentPriceSnapshotNo == "" || directory == nil || directory.SnapshotNo == "" {
		return nil, ErrServiceUnavailable
	}
	var value *accessoryCommerceContext
	if err := s.database.GetScan(ctx, &value, `SELECT price.product_no,price.version_no,
price.billing_rule_no,price.configuration_snapshot_no,product.name_zh_cn,product.name_en_us,
price.tax_policy_no,price.tax_treatment_code,price.tax_rate_basis_points,
price.tax_category_code,price.rounding_scope_code,price.rounding_mode_code,price.rounding_quantum_minor
FROM plugin_idc_core_service_price_snapshot price
JOIN plugin_idc_core_catalog_product product
 ON product.tenant_id=price.tenant_id AND product.product_no=price.product_no
WHERE price.tenant_id=0 AND price.price_snapshot_no=? AND price.service_no=?`,
		service.CurrentPriceSnapshotNo, service.ServiceNo); err != nil {
		return nil, gerror.Wrap(err, "load IP-change commerce context")
	}
	if value == nil {
		return nil, ErrServiceUnavailable
	}
	value.CapabilityEvidenceNo = directory.SnapshotNo
	value.CapabilityEvidenceDigest, _ = digestValue(directory)
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{
		PeriodUnitCode: string(catalog.PeriodUnitOnce), PriceIncludesTax: true,
		TaxTreatmentCode: value.TaxTreatmentCode, TaxRateBasisPoints: value.TaxRateBasisPoints,
		TaxCategoryCode: value.TaxCategoryCode, RoundingScopeCode: value.RoundingScopeCode,
		RoundingModeCode: value.RoundingModeCode, RoundingQuantumMinor: value.RoundingQuantumMinor,
		Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1,
			ChargeAmountMinor: total, PricedAmountMinor: total}},
	}, 0)
	if err != nil || amounts.TotalAmountMinor != total {
		return nil, ErrServiceUnavailable
	}
	value.SubtotalMinor, value.NetAmountMinor, value.TaxAmountMinor =
		amounts.SubtotalMinor, amounts.NetAmountMinor, amounts.TaxAmountMinor
	return value, nil
}
