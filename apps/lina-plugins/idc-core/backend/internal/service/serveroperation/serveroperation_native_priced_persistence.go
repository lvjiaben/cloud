// This file freezes native priced operations into ordinary commerce evidence.
// Identity remains a real entitlement; no legacy instance or channel row is created.

package serveroperation

import (
	"context"
	"encoding/json"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func (s *nativeCommercialService) persistNativePricedQuote(ctx context.Context, q *nativePricedQuote, target *nativeCommercialTarget) error {
	p := q.Price
	if p.ResolvedConfiguration != nil {
		if err := PersistChannelResizeConfiguration(ctx, s.database, p.ResolvedConfiguration, q.TargetConfigurationNo); err != nil {
			return err
		}
	}
	if err := s.persistNativePricedCommerce(ctx, q, target); err != nil {
		return err
	}
	targetJSON, err := json.Marshal(q.Target)
	if err != nil {
		return err
	}
	priceJSON, err := json.Marshal(p)
	if err != nil {
		return err
	}
	var reservation, policy, version, old any
	var targetQuote any
	if p.TargetQuoteNo != "" {
		targetQuote = p.TargetQuoteNo
	}
	if p.Reservation != nil {
		reservation = p.Reservation.ReservationNo
		policy = p.PolicyNo
		version = p.PolicyVersion
		old = p.OldIPSetDigest
	}
	snapshot := q.Target.SnapshotNo
	if p.ResourceSnapshotNo != "" {
		snapshot = p.ResourceSnapshotNo
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_channel_service_quote
 (tenant_id,quote_no,channel_id,payer_customer_no,service_owner_customer_no,service_no,action_code,currency_code,direction_code,
 current_configuration_snapshot_no,target_configuration_snapshot_no,target_version_no,target_billing_rule_no,
 current_recurring_amount_minor,target_recurring_amount_minor,target_subtotal_minor,target_net_amount_minor,target_tax_amount_minor,
 subtotal_minor,net_amount_minor,tax_amount_minor,amount_minor,provider_parameters,resource_channel_no,resource_pool_no,
 resource_reservation_no,resource_snapshot_no,ip_quantity,maximum_snapshot_age_seconds,policy_no,policy_version,old_ip_set_digest,
 expected_service_version,customer_subject_version,payer_customer_subject_version,idempotency_digest,request_digest,pricing_rule_digest,content_digest,
 effective_at,issued_at,expires_at,source_code,native_target_snapshot,prepared_price,native_target_quote_no)
 VALUES(0,?,NULL,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'native',?::JSONB,?::JSONB,?)`,
		q.QuoteNo, p.Context.CustomerNo, p.Context.CustomerNo, p.Context.EntitlementNo, p.Action, p.Context.CurrencyCode, p.Direction,
		p.Context.PriceConfigurationNo, q.TargetConfigurationNo, p.TargetVersionNo, p.TargetBillingRuleNo, p.CurrentRecurringMinor, p.TargetRecurringMinor, p.TargetSubtotalMinor, p.TargetNetMinor, p.TargetTaxMinor,
		p.SubtotalMinor, p.NetAmountMinor, p.TaxAmountMinor, p.TotalAmountMinor, string(p.Parameters), p.Context.ChannelNo, p.Context.PoolNo, reservation, snapshot, p.IPQuantity, p.MaximumSnapshotAgeSeconds, policy, version, old,
		p.Context.EntitlementVersion, p.Context.CustomerVersion, p.Context.CustomerVersion, q.IdempotencyDigest, q.RequestDigest, q.ContentDigest, q.ContentDigest, p.EffectiveAt, p.IssuedAt, p.ExpiresAt, string(targetJSON), string(priceJSON), targetQuote)
	return err
}

func (s *nativeCommercialService) persistNativePricedCommerce(ctx context.Context, q *nativePricedQuote, target *nativeCommercialTarget) error {
	p := q.Price
	cycles := nativePricedPurchaseCycles(p)
	if cycles < 1 {
		return ErrOperationConflict
	}
	itemNo, err := deterministicReference("npi_", q.QuoteNo)
	if err != nil {
		return err
	}
	chargeNo, err := deterministicReference("npb_", q.QuoteNo)
	if err != nil {
		return err
	}
	purpose, chargeType, unit, count, phase := "change", "recurring", p.Context.PeriodUnitCode, p.Context.PeriodCount, "periodic"
	setup, period := int64(0), p.SubtotalMinor
	if p.Action == serveroperationresourcecap.ActionChangeIP {
		purpose, chargeType, unit, count, phase = "ip_change", "one_time", "once", 1, "setup"
		setup, period = p.SubtotalMinor, 0
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_snapshot
 (tenant_id,quote_no,customer_no,customer_subject_version,purpose_code,source_code,currency_code,item_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,input_digest,pricing_rule_digest,eligibility_digest,inventory_digest,content_digest,issued_at,expires_at,created_by)
 VALUES(0,?,?,?,?,'customer',?,1,?,0,?,?,?,?,?,?,?,?,?,?,0)`, q.QuoteNo, p.Context.CustomerNo, p.Context.CustomerVersion, purpose, p.Context.CurrencyCode, p.SubtotalMinor, p.NetAmountMinor, p.TaxAmountMinor, p.TotalAmountMinor, q.RequestDigest, q.ContentDigest, q.ContentDigest, q.ContentDigest, q.ContentDigest, p.IssuedAt, p.ExpiresAt); err != nil {
		return err
	}
	item := do.CommerceQuoteItemSnapshot{TenantId: 0, QuoteItemNo: itemNo, QuoteNo: q.QuoteNo, LineNumber: 1, CurrencyCode: p.Context.CurrencyCode, ProductNo: p.Context.ProductNo, VersionNo: p.TargetVersionNo, BillingRuleNo: p.TargetBillingRuleNo, ConfigurationSnapshotNo: q.TargetConfigurationNo, ProductNameZhCn: target.ProductNameZhCN, ProductNameEnUs: target.ProductNameEnUS, PurchaseCycles: cycles, ChargeTypeCode: chargeType, PeriodUnitCode: unit, PeriodCount: count, FulfillmentModeCode: "automatic", RiskDecisionCode: "approved", InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: q.ContentDigest, TaxPolicyNo: p.Tax.PolicyNo, PriceIncludesTax: p.Tax.PriceIncludesTax, TaxTreatmentCode: p.Tax.Treatment, TaxRateBasisPoints: p.Tax.Rate, TaxCategoryCode: p.Tax.Category, RoundingScopeCode: p.Tax.Scope, RoundingModeCode: p.Tax.Mode, RoundingQuantumMinor: p.Tax.Quantum, SetupSubtotalMinor: setup, PeriodSubtotalMinor: period, SubtotalMinor: p.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: p.NetAmountMinor, TaxAmountMinor: p.TaxAmountMinor, TotalAmountMinor: p.TotalAmountMinor, ContentDigest: q.ContentDigest}
	if _, err = s.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(item).Insert(); err != nil {
		return err
	}
	quantityDigest, err := digestValue(struct {
		ItemNo   string
		Quantity int
	}{itemNo, 1})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_item_quantity_snapshot(tenant_id,quote_item_no,quantity,content_digest) VALUES(0,?,1,?)`, itemNo, quantityDigest); err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot(tenant_id,quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,charge_phase_code,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest) VALUES(0,?,?,?,1,'base',?,1,?,0,?,?)`, chargeNo, itemNo, p.Context.CurrencyCode, phase, p.CatalogChargeMinor, p.CatalogChargeMinor, q.ContentDigest); err != nil {
		return err
	}
	if p.TotalAmountMinor == 0 {
		return nil
	}
	return s.persistNativePricedServicePrice(ctx, q, itemNo, unit, count, phase)
}

func (s *nativeCommercialService) persistNativePricedServicePrice(ctx context.Context, q *nativePricedQuote, itemNo, unit string, count int, chargePhase string) error {
	p := q.Price
	priceNo, err := deterministicReference("nps_", q.QuoteNo)
	if err != nil {
		return err
	}
	phase := "resize_charge"
	if p.Action == serveroperationresourcecap.ActionChangeIP {
		phase = "ip_change"
	}
	components := 1
	if p.TaxAmountMinor > 0 {
		components++
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
 (tenant_id,price_snapshot_no,service_no,subject_type_code,subject_no,customer_no,phase_code,source_quote_no,source_quote_item_no,product_no,version_no,billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,period_unit_code,period_count,renewal_mode_code,capability_evidence_no,capability_evidence_digest,capability_evidence_observed_at,capability_evidence_expires_at,tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,component_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
 VALUES(0,?,NULL,'entitlement',?,?,?,?,?,?,?,?,?,?,?,?,?,'provider_confirmed',?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,?,?,?,?,?)`, priceNo, p.Context.EntitlementNo, p.Context.CustomerNo, phase, q.QuoteNo, itemNo, p.Context.ProductNo, p.TargetVersionNo, p.TargetBillingRuleNo, q.TargetConfigurationNo, p.Context.CurrencyCode, nativePricedPurchaseCycles(p), unit, count, q.QuoteNo, q.ContentDigest, p.IssuedAt, p.ExpiresAt, p.Tax.PolicyNo, p.Tax.PriceIncludesTax, p.Tax.Treatment, nativePriceTaxRate(p.Tax), p.Tax.Category, p.Tax.Scope, p.Tax.Mode, p.Tax.Quantum, components, p.SubtotalMinor, p.NetAmountMinor, p.TaxAmountMinor, p.TotalAmountMinor, q.ContentDigest, q.ContentDigest)
	if err != nil {
		return err
	}
	baseNo, err := deterministicReference("npm_", q.QuoteNo+":base")
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
 (tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,line_number,component_type_code,charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
 VALUES(0,?,?,NULL,'entitlement',?,1,'base',?,?,1,?,?,?,?)`, baseNo, priceNo, p.Context.EntitlementNo, chargePhase, q.QuoteNo, p.NetAmountMinor, p.NetAmountMinor, q.ContentDigest, q.ContentDigest); err != nil {
		return err
	}
	if p.TaxAmountMinor > 0 {
		taxNo, refErr := deterministicReference("npm_", q.QuoteNo+":tax")
		if refErr != nil {
			return refErr
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
 (tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,line_number,component_type_code,charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
 VALUES(0,?,?,NULL,'entitlement',?,2,'tax','total',?,1,?,?,?,?)`, taxNo, priceNo, p.Context.EntitlementNo, p.Tax.PolicyNo, p.TaxAmountMinor, p.TaxAmountMinor, q.ContentDigest, q.ContentDigest)
	}
	return err
}

func nativePriceTaxRate(tax NativePriceTaxContext) any {
	if tax.Rate == nil {
		return nil
	}
	return *tax.Rate
}

func nativePricedPurchaseCycles(price *PreparedNativePrice) int {
	if price.Action == serveroperationresourcecap.ActionResize {
		return price.PurchaseCycles
	}
	return 1
}
