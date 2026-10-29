// This file extracts published configuration pricing, disk safety and integer
// proration from channel admission for reuse by native entitlement admission.
package serveroperation

import (
	"context"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"

	"time"
)

// PrepareResize quotes a supported same-period change. Upgrades charge the
// remaining cycle; safe downgrades are free and effective only at cycle end.
// No configuration is persisted until the caller freezes this quote atomically.
func (s *NativePriceBuilder) PrepareResize(ctx context.Context, c NativePriceContext, input NativePriceResizeInput, now time.Time) (*PreparedNativePrice, error) {
	if !validNativePriceContext(ctx, c, now) || c.PriceConfigurationNo == "" || c.PublicSlug == "" || c.PeriodUnitCode == "once" || c.ExpiresAt == nil || !c.ExpiresAt.After(c.StartedAt) || input.CurrencyCode != c.CurrencyCode || len(input.Selections) == 0 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	public, err := s.catalog.QuoteConfiguration(ctx, catalog.PublicConfigurationQuoteInput{PublicSlug: c.PublicSlug, Revision: input.Revision, CurrencyCode: input.CurrencyCode, Selections: input.Selections})
	if err != nil || public == nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	term := selectNativePriceTerm(public.Terms, input.TermNo, c.PeriodUnitCode, c.PeriodCount)
	if !validNativeResizeTerm(term, c) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	resolver, err := catalog.ServerResizeResolverFromPublicReader(s.catalog)
	if err != nil {
		return nil, err
	}
	resolved, err := resolver.ResolveServerResizeConfiguration(ctx, catalog.ServerResizeConfigurationInput{PublicSlug: c.PublicSlug, Revision: input.Revision, TermNo: term.TermNo, CurrencyCode: input.CurrencyCode, PurchaseCycles: 1, Selections: input.Selections})
	if err != nil || resolved == nil || resolved.ProductNo != c.ProductNo {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var current []catalog.PurchaseConfigurationItem
	if err = s.database.GetScan(ctx, &current, `SELECT group_no,option_no,group_code,option_code,
component_type_code,group_name_zh_cn,group_name_en_us,option_name_zh_cn,option_name_en_us,
normalized_value::TEXT AS normalized_value,quantity,sort_order FROM plugin_idc_core_commerce_configuration_snapshot_item
WHERE tenant_id=0 AND configuration_snapshot_no=? ORDER BY sort_order,id`, c.PriceConfigurationNo); err != nil {
		return nil, err
	}
	parameters, err := PrepareChannelResize(current, resolved.Items)
	if err != nil {
		return nil, err
	}
	validator, ok := s.source.(serveroperationresourcecap.ParameterValidator)
	if !ok || validator.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: c.LocalResourceNo, ChannelNo: c.ChannelNo, PoolNo: c.PoolNo}, serveroperationresourcecap.ActionResize, parameters) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	basis, err := s.paidPeriod(ctx, c)
	if err != nil {
		return nil, err
	}
	c.StartedAt = basis.StartedAt
	recurring := basis.NetAmountMinor
	targetNet, err := nativePublicTermPeriodic(*term, public.Tax)
	if err != nil {
		return nil, err
	}
	amount, effective, direction := int64(0), c.ExpiresAt.UTC(), "downgrade"
	if targetNet > recurring {
		direction = "upgrade"
		effective = now.UTC()
		amount, err = ProrateChannelResize(targetNet-recurring, c.StartedAt, c.ExpiresAt.UTC(), now)
		if err != nil {
			return nil, err
		}
	} else if targetNet == recurring {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	tax, err := s.LoadTax(ctx, resolved.VersionNo, c.CurrencyCode)
	if err != nil {
		return nil, err
	}
	if !nativePriceMatchesTax(*tax, public.Tax) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	// The difference is already tax-exclusive; split no included tax a second
	// time. Tax is calculated once on the prorated paid-period difference.
	tax.PriceIncludesTax = false
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{PeriodUnitCode: string(term.PeriodUnit), PriceIncludesTax: false, TaxTreatmentCode: string(public.Tax.TaxTreatment), TaxRateBasisPoints: public.Tax.TaxRateBasisPoints, TaxCategoryCode: public.Tax.TaxCategoryCode, RoundingScopeCode: string(public.Tax.RoundingScope), RoundingModeCode: string(public.Tax.RoundingMode), RoundingQuantumMinor: public.Tax.RoundingQuantumMinor, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: amount, PricedAmountMinor: amount}}}, 0)
	if err != nil {
		return nil, err
	}
	full, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{PeriodUnitCode: string(term.PeriodUnit), PriceIncludesTax: false, TaxTreatmentCode: tax.Treatment, TaxRateBasisPoints: tax.Rate, TaxCategoryCode: tax.Category, RoundingScopeCode: tax.Scope, RoundingModeCode: tax.Mode, RoundingQuantumMinor: tax.Quantum, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: targetNet, PricedAmountMinor: targetNet}}}, 0)
	if err != nil {
		return nil, err
	}
	return &PreparedNativePrice{Context: c, Action: serveroperationresourcecap.ActionResize, Direction: direction, Parameters: parameters, CurrentRecurringMinor: basis.TotalAmountMinor, CurrentPeriodicNetMinor: recurring, TargetRecurringMinor: full.TotalAmountMinor, CatalogChargeMinor: amount, SubtotalMinor: amounts.SubtotalMinor, NetAmountMinor: amounts.NetAmountMinor, TaxAmountMinor: amounts.TaxAmountMinor, TotalAmountMinor: amounts.TotalAmountMinor, TargetSubtotalMinor: full.SubtotalMinor, TargetNetMinor: full.NetAmountMinor, TargetTaxMinor: full.TaxAmountMinor, TargetVersionNo: resolved.VersionNo, TargetBillingRuleNo: resolved.BillingRuleNo, ResolvedConfiguration: resolved, Tax: *tax, EffectiveAt: effective, IssuedAt: now.UTC(), ExpiresAt: now.UTC().Add(NativePriceQuoteTTL)}, nil
}

func validNativeResizeTerm(term *catalog.PublicQuoteTerm, c NativePriceContext) bool {
	return term != nil && term.TotalAmountMinor != nil && term.NetAmountMinor != nil && term.TaxAmountMinor != nil && term.Status == catalog.PublicQuoteFinal && term.ChargeType == catalog.ChargeTypeRecurring && string(term.PeriodUnit) == c.PeriodUnitCode && term.PeriodCount == c.PeriodCount
}

func nativePublicTermPeriodic(term catalog.PublicQuoteTerm, tax catalog.PublicQuoteTaxSnapshot) (int64, error) {
	line := catalog.PurchaseQuoteLine{PeriodUnitCode: string(term.PeriodUnit), PriceIncludesTax: tax.PriceIncludesTax, TaxTreatmentCode: string(tax.TaxTreatment), TaxRateBasisPoints: tax.TaxRateBasisPoints, TaxCategoryCode: tax.TaxCategoryCode, RoundingScopeCode: string(tax.RoundingScope), RoundingModeCode: string(tax.RoundingMode), RoundingQuantumMinor: tax.RoundingQuantumMinor}
	for _, part := range term.Lines {
		line.Charges = append(line.Charges, catalog.PurchaseQuoteCharge{KindCode: part.Kind, OptionNo: part.OptionNo, Quantity: part.Quantity, ChargeAmountMinor: part.ChargeAmountMinor, SetupFeeMinor: part.SetupFeeMinor, PricedAmountMinor: part.PricedAmountMinor})
	}
	if len(line.Charges) == 0 {
		return 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return commerce.CalculatePeriodicNetAmountWithAdjustments(line, nil)
}

func selectNativePriceTerm(terms []catalog.PublicQuoteTerm, requested, unit string, count int) *catalog.PublicQuoteTerm {
	for i := range terms {
		term := &terms[i]
		if requested != "" && term.TermNo == requested || requested == "" && string(term.PeriodUnit) == unit && term.PeriodCount == count {
			return term
		}
	}
	return nil
}

func nativePriceMatchesTax(tax NativePriceTaxContext, snapshot catalog.PublicQuoteTaxSnapshot) bool {
	rates := tax.Rate == nil && snapshot.TaxRateBasisPoints == nil || tax.Rate != nil && snapshot.TaxRateBasisPoints != nil && *tax.Rate == *snapshot.TaxRateBasisPoints
	return rates && tax.PriceIncludesTax == snapshot.PriceIncludesTax && tax.Treatment == string(snapshot.TaxTreatment) && tax.Category == snapshot.TaxCategoryCode && tax.Scope == string(snapshot.RoundingScope) && tax.Mode == string(snapshot.RoundingMode) && tax.Quantum == snapshot.RoundingQuantumMinor
}
