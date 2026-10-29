// Native procurement does not reserve an upstream price. Recheck its source
// evidence before accepting an immutable local quote, without changing it.
package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strings"
)

type NativeCheckoutPriceVerifier interface {
	VerifyNativeCheckoutPrices(context.Context, PurchaseQuoteInput) (map[string]PurchaseQuoteCharge, error)
}

func (s *publicCatalogReaderImpl) VerifyNativeCheckoutPrices(ctx context.Context, input PurchaseQuoteInput) (map[string]PurchaseQuoteCharge, error) {
	versions := []string{}
	seen := map[string]bool{}
	for _, line := range input.Lines {
		if !seen[line.VersionNo] {
			versions = append(versions, line.VersionNo)
			seen[line.VersionNo] = true
		}
	}
	contracts, err := s.loadNativeImportPricing(ctx, versions)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, nil
	}
	native := PurchaseQuoteInput{CurrencyCode: input.CurrencyCode}
	versionNos := []string{}
	ruleNos := []string{}
	for _, line := range input.Lines {
		if _, found := contracts[line.VersionNo]; found {
			native.Lines = append(native.Lines, line)
			versionNos = append(versionNos, line.VersionNo)
			ruleNos = append(ruleNos, line.BillingRuleNo)
		}
	}
	var rules []entity.CatalogBillingRule
	if err = s.database.GetScan(ctx, &rules, purchaseQuoteBillingRulesSQL, strings.Join(versionNos, ","), strings.Join(ruleNos, ","), len(native.Lines)+1); err != nil {
		return nil, err
	}
	var prices []entity.CatalogPrice
	if err = s.database.GetScan(ctx, &prices, purchaseQuotePricesSQL, strings.Join(versionNos, ","), strings.Join(ruleNos, ","), input.CurrencyCode, "", maximumPurchaseQuotePrices+1); err != nil {
		return nil, err
	}
	if len(prices) > maximumPurchaseQuotePrices {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	native, _, err = s.nativePurchaseQuoteRows(ctx, native, versionNos, rules, prices)
	if err != nil {
		return nil, err
	}
	var taxes []entity.CatalogTaxPolicy
	if err = s.database.GetScan(ctx, &taxes, purchaseQuoteTaxPoliciesSQL, strings.Join(versionNos, ","), input.CurrencyCode, len(contracts)+1); err != nil {
		return nil, err
	}
	taxByVersion := map[string]entity.CatalogTaxPolicy{}
	for _, tax := range taxes {
		taxByVersion[tax.VersionNo] = tax
	}
	lines := make([]PurchaseQuoteLine, 0, len(native.Lines))
	for _, line := range native.Lines {
		tax, found := taxByVersion[line.VersionNo]
		if !found {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		charge, err := nativePurchaseQuoteCharge(line)
		if err != nil {
			return nil, err
		}
		lines = append(lines, PurchaseQuoteLine{ReferenceNo: line.ReferenceNo, VersionNo: line.VersionNo, BillingRuleNo: line.BillingRuleNo, Quantity: line.Quantity, PurchaseCycles: line.PurchaseCycles, PriceIncludesTax: tax.PriceIncludesTax, TaxTreatmentCode: tax.TaxTreatmentCode, TaxRateBasisPoints: tax.TaxRateBasisPoints, RoundingScopeCode: tax.RoundingScopeCode, RoundingModeCode: tax.RoundingModeCode, RoundingQuantumMinor: tax.RoundingQuantumMinor, Charges: []PurchaseQuoteCharge{charge}})
	}
	lines, err = s.applyImportedPurchasePrices(ctx, input.CurrencyCode, versions, lines)
	if err != nil {
		return nil, err
	}
	result := map[string]PurchaseQuoteCharge{}
	for _, line := range lines {
		result[line.ReferenceNo] = line.Charges[0]
	}
	return result, nil
}
