// This file freezes commerce facts for an accessory on the original server.
// Product type comes from that service's authoritative instance, not a legacy
// catalog shadow, so newly published automatic server products remain usable.

package serveroperation

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type accessoryCommerceContext struct {
	ProductNo, VersionNo, BillingRuleNo, ConfigurationSnapshotNo string
	ProductTypeCode, BindingNo                                   string
	TaxPolicyNo, TaxTreatmentCode, TaxCategoryCode               string
	RoundingScopeCode, RoundingModeCode                          string
	TaxRateBasisPoints                                           *int
	RoundingQuantumMinor                                         int64
	CapabilityEvidenceNo, CapabilityEvidenceDigest               string
	NameZhCN, NameEnUS                                           string
	SubtotalMinor, NetAmountMinor, TaxAmountMinor                int64
}

func (s *accessoryPricingImpl) loadAccessoryCommerceContext(ctx context.Context, service *serviceRow,
	directory *serveroperationresourcecap.AccessoryProjection, kind serveroperationresourcecap.AccessoryKind,
	total int64) (*accessoryCommerceContext, error) {
	if service.CurrentPriceSnapshotNo == "" || directory.SnapshotNo == "" {
		return nil, ErrServiceUnavailable
	}
	var result *accessoryCommerceContext
	if err := s.database.GetScan(ctx, &result, `SELECT price.product_no,price.version_no,
price.billing_rule_no,price.configuration_snapshot_no,service.product_type_code,
service.binding_no,price.tax_policy_no,price.tax_treatment_code,price.tax_rate_basis_points,
price.tax_category_code,price.rounding_scope_code,price.rounding_mode_code,price.rounding_quantum_minor
    ,accessory_kind.name_zh_cn,accessory_kind.name_en_us
FROM plugin_idc_core_service_price_snapshot price
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id=price.tenant_id AND service.service_no=price.service_no
JOIN plugin_idc_core_catalog_product product
 ON product.tenant_id=price.tenant_id AND product.product_no=price.product_no
JOIN plugin_idc_core_server_accessory_kind accessory_kind
 ON accessory_kind.accessory_kind=?
WHERE price.tenant_id=0 AND price.price_snapshot_no=? AND price.service_no=?`,
		kind, service.CurrentPriceSnapshotNo, service.ServiceNo); err != nil {
		return nil, gerror.Wrap(err, "load accessory commerce context")
	}
	if result == nil {
		return nil, ErrServiceUnavailable
	}
	capabilityDigest, err := digestValue(directory)
	if err != nil {
		return nil, err
	}
	result.CapabilityEvidenceNo = directory.SnapshotNo
	result.CapabilityEvidenceDigest = capabilityDigest
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{
		PeriodUnitCode: string(accessoryPeriodUnit(kind)), PriceIncludesTax: true,
		TaxTreatmentCode: result.TaxTreatmentCode, TaxRateBasisPoints: result.TaxRateBasisPoints,
		TaxCategoryCode: result.TaxCategoryCode, RoundingScopeCode: result.RoundingScopeCode,
		RoundingModeCode: result.RoundingModeCode, RoundingQuantumMinor: result.RoundingQuantumMinor,
		Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1,
			ChargeAmountMinor: total, PricedAmountMinor: total}},
	}, 0)
	if err != nil || amounts.TotalAmountMinor != total {
		return nil, ErrServiceUnavailable
	}
	result.SubtotalMinor, result.NetAmountMinor, result.TaxAmountMinor =
		amounts.SubtotalMinor, amounts.NetAmountMinor, amounts.TaxAmountMinor
	return result, nil
}

func accessoryPeriodUnit(kind serveroperationresourcecap.AccessoryKind) catalog.PeriodUnit {
	if kind == serveroperationresourcecap.AccessoryKindTraffic {
		return catalog.PeriodUnitOnce
	}
	return catalog.PeriodUnitMonth
}

func accessoryTaxValue(value *accessoryCommerceContext, field string) any {
	if value == nil {
		return nil
	}
	switch field {
	case "policy":
		return value.TaxPolicyNo
	case "treatment":
		return value.TaxTreatmentCode
	case "category":
		return value.TaxCategoryCode
	case "scope":
		return value.RoundingScopeCode
	case "mode":
		return value.RoundingModeCode
	default:
		return nil
	}
}

func accessoryQuoteTaxValue(quote *AccessoryQuote, value *accessoryCommerceContext, field string) any {
	if quote.TotalAmountMinor == 0 {
		return nil
	}
	return accessoryTaxValue(value, field)
}

func accessoryTaxRate(value *accessoryCommerceContext) any {
	if value == nil || value.TaxRateBasisPoints == nil {
		return nil
	}
	return *value.TaxRateBasisPoints
}

func accessoryQuoteTaxRate(quote *AccessoryQuote, value *accessoryCommerceContext) any {
	if quote.TotalAmountMinor == 0 {
		return nil
	}
	return accessoryTaxRate(value)
}

func accessoryTaxQuantum(value *accessoryCommerceContext) any {
	if value == nil {
		return nil
	}
	return value.RoundingQuantumMinor
}

func accessoryQuoteTaxQuantum(quote *AccessoryQuote, value *accessoryCommerceContext) any {
	if quote.TotalAmountMinor == 0 {
		return nil
	}
	return accessoryTaxQuantum(value)
}

func (s *accessoryPricingImpl) persistAccessoryQuoteCommerce(ctx context.Context, quote *AccessoryQuote,
	parameters json.RawMessage, requestDigest, accessoryDigest string, value *accessoryCommerceContext) error {
	quoteItemNo, err := deterministicReference("aqi_", quote.QuoteNo)
	if err != nil {
		return err
	}
	quoteChargeNo, err := deterministicReference("aqc_", quote.QuoteNo)
	if err != nil {
		return err
	}
	priceNo, err := deterministicReference("aps_", quote.QuoteNo)
	if err != nil {
		return err
	}
	componentNo, err := deterministicReference("apm_", quote.QuoteNo+":base")
	if err != nil {
		return err
	}
	pricingDigest, err := digestValue(struct {
		PolicyNo string
		Version  int64
		Total    int64
	}{quote.PolicyNo, quote.PolicyVersion, quote.TotalAmountMinor})
	if err != nil {
		return err
	}
	eligibilityDigest, err := digestValue(struct {
		CustomerNo, ServiceNo           string
		CustomerVersion, ServiceVersion int64
	}{quote.CustomerNo, quote.ServiceNo, quote.CustomerSubjectVersion, quote.ExpectedServiceVersion})
	if err != nil {
		return err
	}
	nameZh, nameEn := value.NameZhCN, value.NameEnUS
	chargeType, chargePhase := "recurring", "periodic"
	setupSubtotal, periodSubtotal := int64(0), quote.SubtotalMinor
	if quote.Kind == serveroperationresourcecap.AccessoryKindTraffic {
		chargeType, chargePhase = "one_time", "setup"
		setupSubtotal, periodSubtotal = quote.SubtotalMinor, 0
	}
	quoteContent, err := digestValue(struct {
		QuoteNo, CustomerNo, Currency, Input, Pricing, Eligibility, Inventory string
		Total                                                                 int64
	}{quote.QuoteNo, quote.CustomerNo, quote.CurrencyCode, requestDigest, pricingDigest,
		eligibilityDigest, value.CapabilityEvidenceDigest, quote.TotalAmountMinor})
	if err != nil {
		return err
	}
	itemContent, err := digestValue(struct {
		QuoteItemNo, QuoteNo, ServiceNo, Kind string
		Parameters                            json.RawMessage
		Total                                 int64
	}{quoteItemNo, quote.QuoteNo, quote.ServiceNo, string(quote.Kind), parameters, quote.TotalAmountMinor})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_snapshot
(tenant_id,quote_no,customer_no,customer_subject_version,purpose_code,source_code,currency_code,item_count,
subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,input_digest,
pricing_rule_digest,eligibility_digest,inventory_digest,content_digest,issued_at,expires_at,created_by)
VALUES (0,?,?,?,'addon','customer',?,1,?,0,?,?,?,?,?,?,?,?,?,?,0)`, quote.QuoteNo, quote.CustomerNo,
		quote.CustomerSubjectVersion, quote.CurrencyCode, quote.SubtotalMinor, quote.NetAmountMinor,
		quote.TaxAmountMinor, quote.TotalAmountMinor, requestDigest, pricingDigest, eligibilityDigest,
		value.CapabilityEvidenceDigest, quoteContent, quote.IssuedAt, quote.ExpiresAt); err != nil {
		return gerror.Wrap(err, "insert accessory commerce quote")
	}
	item := do.CommerceQuoteItemSnapshot{
		TenantId: 0, QuoteItemNo: quoteItemNo, QuoteNo: quote.QuoteNo, LineNumber: 1,
		CurrencyCode: quote.CurrencyCode, ProductNo: value.ProductNo, VersionNo: value.VersionNo,
		BillingRuleNo: value.BillingRuleNo, ConfigurationSnapshotNo: value.ConfigurationSnapshotNo,
		ProductNameZhCn: nameZh, ProductNameEnUs: nameEn, PurchaseCycles: 1,
		ChargeTypeCode: chargeType, PeriodUnitCode: quote.PeriodUnitCode, PeriodCount: quote.PeriodCount,
		FulfillmentModeCode: "automatic", RiskDecisionCode: "approved",
		InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: value.CapabilityEvidenceDigest,
		TaxPolicyNo: value.TaxPolicyNo, PriceIncludesTax: true, TaxTreatmentCode: value.TaxTreatmentCode,
		TaxRateBasisPoints: value.TaxRateBasisPoints, TaxCategoryCode: value.TaxCategoryCode,
		RoundingScopeCode: value.RoundingScopeCode, RoundingModeCode: value.RoundingModeCode,
		RoundingQuantumMinor: value.RoundingQuantumMinor, SetupSubtotalMinor: setupSubtotal,
		PeriodSubtotalMinor: periodSubtotal, SubtotalMinor: quote.SubtotalMinor, DiscountMinor: 0,
		NetAmountMinor: quote.NetAmountMinor, TaxAmountMinor: quote.TaxAmountMinor,
		TotalAmountMinor: quote.TotalAmountMinor, ContentDigest: itemContent,
	}
	if _, err = s.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(item).Insert(); err != nil {
		return gerror.Wrap(err, "insert accessory commerce quote item")
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
		return gerror.Wrap(err, "insert accessory commerce quote quantity")
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot
(tenant_id,quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,charge_phase_code,
quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest)
VALUES (0,?,?,?,1,'base',?,1,?,0,?,?)`, quoteChargeNo, quoteItemNo, quote.CurrencyCode,
		chargePhase, quote.TotalAmountMinor, quote.TotalAmountMinor, pricingDigest); err != nil {
		return gerror.Wrap(err, "insert accessory commerce quote charge")
	}
	if quote.TotalAmountMinor == 0 {
		return nil
	}
	priceContent, err := digestValue(struct {
		PriceNo, QuoteNo, QuoteItemNo, ServiceNo, Evidence string
		Total                                              int64
	}{priceNo, quote.QuoteNo, quoteItemNo, quote.ServiceNo, value.CapabilityEvidenceDigest, quote.TotalAmountMinor})
	if err != nil {
		return err
	}
	componentCount := 1
	if quote.TaxAmountMinor > 0 {
		componentCount++
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
(tenant_id,price_snapshot_no,service_no,subject_type_code,subject_no,customer_no,phase_code,source_quote_no,source_quote_item_no,
product_no,version_no,billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,
period_unit_code,period_count,renewal_mode_code,capability_evidence_no,capability_evidence_digest,
capability_evidence_observed_at,capability_evidence_expires_at,original_expires_at,projected_expires_at,
tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,
rounding_scope_code,rounding_mode_code,rounding_quantum_minor,component_count,subtotal_minor,
discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
VALUES (0,?,?,?,?,?,'addon',?,?,?,?,?,?,?,?,?,?,'provider_confirmed',?,?,?, ?,NULL,NULL,?,TRUE,?,?,?,?,?,?,
?, ?,0,?,?,?,?,?)`, priceNo, accessoryLegacyService(quote), accessorySubjectType(quote), accessorySubjectNo(quote), quote.CustomerNo, quote.QuoteNo, quoteItemNo,
		value.ProductNo, value.VersionNo, value.BillingRuleNo, value.ConfigurationSnapshotNo, quote.CurrencyCode,
		1, quote.PeriodUnitCode, quote.PeriodCount, value.CapabilityEvidenceNo,
		value.CapabilityEvidenceDigest, quote.IssuedAt, quote.ExpiresAt, value.TaxPolicyNo,
		value.TaxTreatmentCode, value.TaxRateBasisPoints, value.TaxCategoryCode, value.RoundingScopeCode,
		value.RoundingModeCode, value.RoundingQuantumMinor, componentCount, quote.SubtotalMinor,
		quote.NetAmountMinor, quote.TaxAmountMinor, quote.TotalAmountMinor, pricingDigest, priceContent); err != nil {
		return gerror.Wrap(err, "insert accessory service price snapshot")
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,line_number,component_type_code,
charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES (0,?,?,?,?,?,1,'base',?,?,1,?,?,?,?)`, componentNo, priceNo, accessoryLegacyService(quote), accessorySubjectType(quote), accessorySubjectNo(quote), chargePhase,
		quote.PolicyNo, quote.NetAmountMinor, quote.NetAmountMinor, pricingDigest, itemContent); err != nil {
		return gerror.Wrap(err, "insert accessory base price component")
	}
	if quote.TaxAmountMinor > 0 {
		taxComponentNo, refErr := deterministicReference("apm_", quote.QuoteNo+":tax")
		if refErr != nil {
			return refErr
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,line_number,component_type_code,
charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES (0,?,?,?,?,?,2,'tax','total',?,1,?,?,?,?)`, taxComponentNo, priceNo, accessoryLegacyService(quote), accessorySubjectType(quote), accessorySubjectNo(quote),
			value.TaxPolicyNo, quote.TaxAmountMinor, quote.TaxAmountMinor, pricingDigest, accessoryDigest); err != nil {
			return gerror.Wrap(err, "insert accessory tax price component")
		}
	}
	return nil
}
