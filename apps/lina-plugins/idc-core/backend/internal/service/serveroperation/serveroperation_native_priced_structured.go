// Native structured resize consumes a real owned change quote and its typed
// configuration. Supplier parameters are produced only by the resource owner.
package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/entitlementupgrade"
)

type nativeStructuredPriceTarget struct {
	QuoteNo, QuoteItemNo, ConfigurationNo, StructuredNo             string
	ProductConfiguration, CurrentConfiguration, TargetConfiguration string
	ExpiresAt                                                       time.Time
	Line                                                            catalog.PurchaseQuoteLine
	PeriodNet                                                       int64
}

func (s *nativeCommercialService) nativeStructuredResize(ctx context.Context, target *nativeCommercialTarget, snapshot servercommercialcap.TargetSnapshot, input *servercommercialcap.ResizeInput, now time.Time) (*PreparedNativePrice, string, error) {
	if input == nil || !validReference(input.TargetQuoteNo, 40) || input.Revision != "" || input.TermNo != "" || input.CurrencyCode != "" || len(input.Selections) != 0 || target.ExpiresAt == nil || target.PeriodUnitCode == "once" {
		return nil, "", ErrInvalidInput
	}
	to, err := s.readNativeStructuredPriceTarget(ctx, target, snapshot, input.TargetQuoteNo, now)
	if err != nil {
		return nil, "", err
	}
	compiler, ok := s.prices.source.(serveroperationresourcecap.UpgradeConfigurationCompiler)
	if !ok || nilServerOperationDependency(compiler) {
		return nil, "", ErrActionUnavailable
	}
	compiled, err := compiler.CompileUpgrade(ctx, serveroperationresourcecap.UpgradeConfigurationRequest{ServiceNo: target.EntitlementNo, CustomerNo: target.CustomerNo, ProductConfiguration: json.RawMessage(to.ProductConfiguration), CurrentConfiguration: json.RawMessage(to.CurrentConfiguration), TargetConfiguration: json.RawMessage(to.TargetConfiguration)})
	if err != nil {
		return nil, "", err
	}
	if compiled == nil || compiled.Route.ServiceNo != target.LocalResourceNo || compiled.Route.ChannelNo != target.ChannelNo || compiled.Route.PoolNo != target.PoolNo || len(compiled.Parameters) == 0 || len(compiled.Parameters) > 65536 {
		return nil, "", ErrOperationConflict
	}
	basis, err := s.prices.paidPeriod(ctx, target.NativePriceContext)
	if err != nil {
		return nil, "", err
	}
	// The paid upgrade owner rejects negative differences and rounds the net
	// remaining-period difference once; tax is deliberately applied afterwards.
	charge, err := entitlementupgrade.ProrateDifference(entitlementupgrade.PeriodPrice{ActualMinor: basis.NetAmountMinor, TargetMinor: to.PeriodNet, StartedAt: basis.StartedAt, ExpiresAt: target.ExpiresAt.UTC(), Now: now})
	if err != nil {
		return nil, "", err
	}
	line := to.Line
	line.PriceIncludesTax = false
	line.Charges = []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: charge, PricedAmountMinor: charge}}
	amounts, err := commerce.CalculatePricingAmounts(line, 0)
	if err != nil {
		return nil, "", err
	}
	line.Charges = []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: to.PeriodNet, PricedAmountMinor: to.PeriodNet}}
	full, err := commerce.CalculatePricingAmounts(line, 0)
	if err != nil {
		return nil, "", err
	}
	c := target.NativePriceContext
	c.StartedAt = basis.StartedAt
	expires := now.Add(NativePriceQuoteTTL)
	if to.ExpiresAt.Before(expires) {
		expires = to.ExpiresAt
	}
	return &PreparedNativePrice{Context: c, Action: serveroperationresourcecap.ActionResize, Direction: "upgrade", Parameters: compiled.Parameters,
		CurrentRecurringMinor: basis.TotalAmountMinor, CurrentPeriodicNetMinor: basis.NetAmountMinor, TargetRecurringMinor: full.TotalAmountMinor, CatalogChargeMinor: charge,
		SubtotalMinor: amounts.SubtotalMinor, NetAmountMinor: amounts.NetAmountMinor, TaxAmountMinor: amounts.TaxAmountMinor, TotalAmountMinor: amounts.TotalAmountMinor,
		TargetSubtotalMinor: full.SubtotalMinor, TargetNetMinor: full.NetAmountMinor, TargetTaxMinor: full.TaxAmountMinor,
		TargetVersionNo: line.VersionNo, TargetBillingRuleNo: line.BillingRuleNo, TargetQuoteNo: to.QuoteNo, TargetQuoteItemNo: to.QuoteItemNo, TargetStructuredConfigurationNo: to.StructuredNo, PurchaseCycles: line.PurchaseCycles,
		Tax: NativePriceTaxContext{PolicyNo: line.TaxPolicyNo, Treatment: line.TaxTreatmentCode, Category: line.TaxCategoryCode, Scope: line.RoundingScopeCode, Mode: line.RoundingModeCode, Rate: line.TaxRateBasisPoints, Quantum: line.RoundingQuantumMinor, PriceIncludesTax: false}, EffectiveAt: now.UTC(), IssuedAt: now.UTC(), ExpiresAt: expires}, to.ConfigurationNo, nil
}

func (s *nativeCommercialService) readNativeStructuredPriceTarget(ctx context.Context, target *nativeCommercialTarget, snapshot servercommercialcap.TargetSnapshot, quoteNo string, now time.Time) (*nativeStructuredPriceTarget, error) {
	var row *nativeStructuredPriceTarget
	err := s.database.GetScan(ctx, &row, `SELECT q.quote_no,i.quote_item_no,i.configuration_snapshot_no AS configuration_no,
 sc.snapshot_no AS structured_no,sc.typed_value_snapshot::TEXT AS target_configuration,q.expires_at,
 vc.merchant_value_snapshot::TEXT AS product_configuration,current_sc.typed_value_snapshot::TEXT AS current_configuration
 FROM plugin_idc_core_native_server_target_quote binding
 JOIN plugin_idc_core_commerce_quote_snapshot q ON q.tenant_id=binding.tenant_id AND q.quote_no=binding.target_quote_no
 JOIN plugin_idc_core_commerce_quote_item_snapshot i ON i.tenant_id=q.tenant_id AND i.quote_no=q.quote_no AND i.quote_item_no=binding.quote_item_no
 JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc ON sc.tenant_id=i.tenant_id AND sc.quote_item_no=i.quote_item_no AND sc.snapshot_no=binding.structured_configuration_no
 JOIN plugin_idc_core_catalog_product_version_configuration vc ON vc.tenant_id=i.tenant_id AND vc.version_no=i.version_no
 JOIN plugin_idc_core_commerce_structured_configuration_snapshot current_sc ON current_sc.tenant_id=i.tenant_id AND current_sc.snapshot_no=? AND current_sc.customer_no=?
 WHERE binding.tenant_id=0 AND binding.target_quote_no=? AND binding.entitlement_no=? AND binding.expected_entitlement_version=?
 AND binding.customer_no=? AND binding.customer_subject_version=? AND binding.currency_code=?
 AND binding.native_target_snapshot->>'LocalResourceNo'=? AND binding.native_target_snapshot->>'ChannelNo'=? AND binding.native_target_snapshot->>'PoolNo'=?
 AND q.customer_no=binding.customer_no AND q.customer_subject_version=binding.customer_subject_version AND q.purpose_code='change' AND q.source_code IN ('customer','staff') AND q.created_by>0 AND q.item_count=1
 AND i.product_no=? AND i.version_no=? AND i.currency_code=binding.currency_code AND i.charge_type_code='recurring'
 AND sc.customer_no=binding.customer_no AND sc.order_no IS NULL
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_structured_configuration_file_snapshot f WHERE f.snapshot_no=sc.snapshot_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_order o WHERE o.quote_no=q.quote_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_addon_target a WHERE a.quote_no=q.quote_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_quote_adjustment_snapshot a WHERE a.quote_item_no=i.quote_item_no AND a.reservation_reference IS NOT NULL)
 FOR SHARE OF binding,q,i,sc,vc,current_sc`, target.StructuredConfigurationNo, target.CustomerNo, quoteNo, target.EntitlementNo, target.EntitlementVersion, target.CustomerNo, target.CustomerVersion, target.CurrencyCode, snapshot.LocalResourceNo, snapshot.ChannelNo, snapshot.PoolNo, target.ProductNo, target.VersionNo)
	if err != nil {
		return nil, err
	}
	if row == nil || !row.ExpiresAt.After(now) {
		return nil, ErrOperationConflict
	}
	row.Line, row.PeriodNet, err = s.prices.periodicQuoteNet(ctx, row.QuoteItemNo)
	if err != nil {
		return nil, err
	}
	if row.Line.Quantity != 1 || row.Line.PurchaseCycles != target.PurchaseCycles || row.Line.PeriodUnitCode != target.PeriodUnitCode || row.Line.PeriodCount != target.PeriodCount || row.PeriodNet < 0 {
		return nil, ErrOperationConflict
	}
	return row, nil
}
