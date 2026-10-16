// This file applies immutable imported whole-item rules after configuration
// pricing and before customer promotions. Setup fees remain independent; all
// arithmetic uses integer minor units and deterministic remainder allocation.

package catalog

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/model/do"
)

type importedWholePriceRow struct {
	VersionNo, BillingRuleNo, CurrencyCode string
	WholeItemRule                          string
}

func (s *publicCatalogReaderImpl) importedWholePrices(ctx context.Context, versions []string, currency string) (map[string]channelcommercecap.ImportWholeItemRule, error) {
	var rows []importedWholePriceRow
	filter := do.CatalogImportTerm{TenantId: 0}
	if currency != "" {
		filter.CurrencyCode = currency
	}
	err := s.database.Model(do.CatalogImportTerm{}).Ctx(ctx).Where(filter).WhereIn("version_no", versions).
		Fields("version_no,billing_rule_no,currency_code,whole_item_rule").Limit(maximumPublicationPrices + 1).Scan(&rows)
	if err != nil || len(rows) > maximumPublicationPrices {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	result := make(map[string]channelcommercecap.ImportWholeItemRule)
	for _, row := range rows {
		if row.WholeItemRule == "" || row.WholeItemRule == "null" {
			continue
		}
		var rule channelcommercecap.ImportWholeItemRule
		if json.Unmarshal([]byte(row.WholeItemRule), &rule) != nil || rule.Validate() != nil {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		result[importWholePriceKey(row.VersionNo, row.BillingRuleNo, row.CurrencyCode)] = rule
	}
	return result, nil
}

func (s *publicCatalogReaderImpl) applyImportedPurchasePrices(ctx context.Context, currency string, versions []string, lines []PurchaseQuoteLine) ([]PurchaseQuoteLine, error) {
	rules, err := s.importedWholePrices(ctx, versions, currency)
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		rule, exists := rules[importWholePriceKey(line.VersionNo, line.BillingRuleNo, currency)]
		if !exists {
			continue
		}
		lines[i], err = applyImportedWholePrice(line, rule)
		if err != nil {
			return nil, err
		}
	}
	return lines, nil
}

func (s *publicCatalogReaderImpl) applyImportedPublicPrices(ctx context.Context, version string, quote *PublicConfigurationQuote) error {
	rules, err := s.importedWholePrices(ctx, []string{version}, quote.CurrencyCode)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}
	// A public quote has one tax flag for all terms. Normalize every displayed
	// term to net components, including terms without an imported rule, so the
	// flag cannot describe mixed gross and net amounts. Untouched term totals
	// retain the original rounding result instead of a lossy tax round trip.
	for i := range quote.Terms {
		term := &quote.Terms[i]
		if len(term.Lines) == 0 {
			continue
		}
		rule, exists := rules[importWholePriceKey(version, term.TermNo, quote.CurrencyCode)]
		applyRule := exists && term.Status == PublicQuoteFinal
		if !applyRule {
			zero := 0
			rule = channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &zero}
		}
		line := PurchaseQuoteLine{Quantity: 1, PurchaseCycles: term.Cycles, PriceIncludesTax: quote.Tax.PriceIncludesTax,
			TaxTreatmentCode: string(quote.Tax.TaxTreatment), TaxRateBasisPoints: quote.Tax.TaxRateBasisPoints,
			RoundingScopeCode: string(quote.Tax.RoundingScope), RoundingModeCode: string(quote.Tax.RoundingMode), RoundingQuantumMinor: quote.Tax.RoundingQuantumMinor,
			Charges: make([]PurchaseQuoteCharge, len(term.Lines))}
		for j, c := range term.Lines {
			line.Charges[j] = PurchaseQuoteCharge{ChargeAmountMinor: c.ChargeAmountMinor, SetupFeeMinor: c.SetupFeeMinor, PricedAmountMinor: c.PricedAmountMinor}
		}
		adjusted, err := applyImportedWholePrice(line, rule)
		if err != nil {
			return err
		}
		for j, c := range adjusted.Charges {
			term.Lines[j].ChargeAmountMinor = c.ChargeAmountMinor
			term.Lines[j].SetupFeeMinor = c.SetupFeeMinor
			term.Lines[j].PricedAmountMinor = c.PricedAmountMinor
		}
		if !applyRule {
			continue
		}
		policy := TaxPolicyDefinition{PriceIncludesTax: false, TaxTreatment: quote.Tax.TaxTreatment, TaxRateBasisPoints: quote.Tax.TaxRateBasisPoints,
			TaxCategoryCode: quote.Tax.TaxCategoryCode, RoundingScope: quote.Tax.RoundingScope, RoundingMode: quote.Tax.RoundingMode, RoundingQuantumMinor: quote.Tax.RoundingQuantumMinor}
		net, tax, total, err := calculatePublicQuoteTax(term.Lines, policy)
		if err != nil {
			return err
		}
		term.NetAmountMinor, term.TaxAmountMinor, term.TotalAmountMinor = &net, &tax, &total
	}
	quote.Tax.PriceIncludesTax = false
	return nil
}

// importWholePriceKey identifies an exact immutable version, rule and currency.
func importWholePriceKey(version, rule, currency string) string {
	return version + "\x00" + rule + "\x00" + currency
}

func applyImportedWholePrice(line PurchaseQuoteLine, rule channelcommercecap.ImportWholeItemRule) (PurchaseQuoteLine, error) {
	if rule.Validate() != nil {
		return PurchaseQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	chain := make([]*channelcommercecap.ImportWholeItemRule, 0, channelcommercecap.MaximumImportWholeItemRuleDepth)
	for current := &rule; current != nil; current = current.Prior {
		chain = append(chain, current)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		step := *chain[i]
		step.Prior = nil
		var err error
		line, err = applyImportedWholePriceStep(line, step)
		if err != nil {
			return PurchaseQuoteLine{}, err
		}
	}
	return line, nil
}

func applyImportedWholePriceStep(line PurchaseQuoteLine, rule channelcommercecap.ImportWholeItemRule) (PurchaseQuoteLine, error) {
	if len(line.Charges) == 0 || line.Quantity < 1 || line.PurchaseCycles < 1 {
		return PurchaseQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	weights := make([]int64, len(line.Charges)*2)
	for i, charge := range line.Charges {
		weights[2*i], weights[2*i+1] = charge.SetupFeeMinor, charge.ChargeAmountMinor
	}
	net, err := importedNetComponents(line, weights)
	if err != nil {
		return PurchaseQuoteLine{}, err
	}
	period := make([]int64, len(line.Charges))
	var total int64
	for i := range period {
		period[i] = net[2*i+1]
		total, err = addBillingAmount(total, period[i])
		if err != nil {
			return PurchaseQuoteLine{}, err
		}
	}
	target := big.NewInt(total)
	switch rule.Kind {
	case "fixed":
		if rule.FixedAmountMinor == nil || *rule.FixedAmountMinor < 0 || rule.DiscountBasisPoints != nil {
			return PurchaseQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		target.Mul(big.NewInt(*rule.FixedAmountMinor), big.NewInt(int64(line.Quantity)))
		target.Mul(target, big.NewInt(int64(line.PurchaseCycles)))
	case "discount":
		if rule.DiscountBasisPoints == nil || *rule.DiscountBasisPoints < 0 || *rule.DiscountBasisPoints > 10000 || rule.FixedAmountMinor != nil {
			return PurchaseQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		target.Mul(target, big.NewInt(int64(10000-*rule.DiscountBasisPoints)))
		target.Quo(target, big.NewInt(10000))
	default:
		return PurchaseQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	if !target.IsInt64() {
		return PurchaseQuoteLine{}, bizerr.NewCode(CodeBillingAmountOverflow)
	}
	allocated, err := allocateImportedAmount(target.Int64(), period)
	if err != nil {
		return PurchaseQuoteLine{}, err
	}
	line.Charges = append([]PurchaseQuoteCharge(nil), line.Charges...)
	for i := range line.Charges {
		charge := &line.Charges[i]
		charge.ChargeAmountMinor, charge.SetupFeeMinor = allocated[i], net[2*i]
		charge.PricedAmountMinor, err = addBillingAmount(charge.ChargeAmountMinor, charge.SetupFeeMinor)
		if err != nil {
			return PurchaseQuoteLine{}, err
		}
		charge.PriceSourceDigest, err = purchaseQuoteDigest(struct {
			Prior         string
			Rule          channelcommercecap.ImportWholeItemRule
			Period, Setup int64
		}{charge.PriceSourceDigest, rule, charge.ChargeAmountMinor, charge.SetupFeeMinor})
		if err != nil {
			return PurchaseQuoteLine{}, err
		}
	}
	line.PriceIncludesTax = false
	return line, nil
}

func importedNetComponents(line PurchaseQuoteLine, weights []int64) ([]int64, error) {
	net := append([]int64(nil), weights...)
	if !line.PriceIncludesTax || line.TaxTreatmentCode != string(TaxTreatmentTaxable) {
		return net, nil
	}
	if line.TaxRateBasisPoints == nil {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	groups := [][]int64{weights}
	if line.RoundingScopeCode == string(TaxRoundingScopeLine) {
		groups = make([][]int64, len(line.Charges))
		for i := range groups {
			groups[i] = weights[2*i : 2*i+2]
		}
	}
	offset := 0
	for _, group := range groups {
		var total int64
		for _, amount := range group {
			var err error
			total, err = addBillingAmount(total, amount)
			if err != nil {
				return nil, err
			}
		}
		rate := int64(*line.TaxRateBasisPoints)
		tax, err := RoundTaxRational(total, rate, 10000+rate, line.RoundingQuantumMinor, TaxRoundingMode(line.RoundingModeCode))
		if err != nil || tax > total {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		allocated, err := allocateImportedAmount(tax, group)
		if err != nil {
			return nil, err
		}
		for i, value := range allocated {
			net[offset+i] -= value
		}
		offset += len(group)
	}
	return net, nil
}

func allocateImportedAmount(target int64, weights []int64) ([]int64, error) {
	if target < 0 || len(weights) == 0 {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	var total int64
	for _, value := range weights {
		var err error
		total, err = addBillingAmount(total, value)
		if err != nil {
			return nil, err
		}
	}
	result := make([]int64, len(weights))
	if total == 0 {
		result[0] = target
		return result, nil
	}
	type remainder struct {
		index  int
		amount *big.Int
	}
	remainders := make([]remainder, len(weights))
	var allocated int64
	for i, value := range weights {
		numerator := new(big.Int).Mul(big.NewInt(target), big.NewInt(value))
		quotient, rest := new(big.Int), new(big.Int)
		quotient.QuoRem(numerator, big.NewInt(total), rest)
		result[i] = quotient.Int64()
		allocated += result[i]
		remainders[i] = remainder{i, rest}
	}
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].amount.Cmp(remainders[j].amount) > 0 })
	for i := int64(0); i < target-allocated; i++ {
		result[remainders[i].index]++
	}
	return result, nil
}
