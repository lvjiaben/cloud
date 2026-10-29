// Imported native prices are read for the selected compiler values. A local
// schedule is never a substitute for the authenticated account's live price.
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type ImportedBillingInput struct {
	VersionNo, CurrencyCode string
	Values                  []configurationcontract.ConfigurationValue
}
type ImportedBillingRuntime interface {
	ReadImportedPurchaseBillingTerms(context.Context, []ImportedBillingInput) ([]*channelcommercecap.BillingTerms, error)
}
type nativeImportPricingContract struct {
	Snapshot channelcommercecap.ImportSnapshot
	Read     resourceinterfaceprojectioncap.CatalogReadEvidence
}

func (s *publicCatalogReaderImpl) loadNativeImportPricing(ctx context.Context, versions []string) (map[string]nativeImportPricingContract, error) {
	var rows []struct{ VersionNo, Snapshot, SnapshotDigest, ReadEvidence string }
	if err := s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0}).WhereIn("version_no", versions).Fields("version_no,snapshot,snapshot_digest,read_evidence").Limit(len(versions) + 1).Scan(&rows); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	if len(rows) > len(versions) {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	result := map[string]nativeImportPricingContract{}
	for _, row := range rows {
		var contract nativeImportPricingContract
		if json.Unmarshal([]byte(row.Snapshot), &contract.Snapshot) != nil || contract.Snapshot.Validate() != nil || validateImportCurrencyPrecision(&contract.Snapshot) != nil || json.Unmarshal([]byte(row.ReadEvidence), &contract.Read) != nil || contract.Read.Validate() != nil {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		raw, err := json.Marshal(contract.Snapshot)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != row.SnapshotDigest {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		if contract.Read.SaleImportProtocol.NativePurchase() {
			result[row.VersionNo] = contract
		}
		if contract.Snapshot.PricingMode == channelcommercecap.ImportPricingRemoteEvaluation && !contract.Read.SaleImportProtocol.NativePurchase() {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
	}
	return result, nil
}

// quoteStructuredPurchasePrices shares the complete native path between public
// display and cart resolution. Ordinary products retain local integer prices.
func (s *publicCatalogReaderImpl) quoteStructuredPurchasePrices(ctx context.Context, version, revision, currency string, source *publicQuoteSource, selections map[string]int, values []configurationcontract.ConfigurationValue) (*PublicConfigurationQuote, error) {
	contracts, err := s.loadNativeImportPricing(ctx, []string{version})
	if err != nil {
		return nil, err
	}
	contract, exists := contracts[version]
	if !exists {
		return buildPublicConfigurationQuote(revision, currency, source, selections)
	}
	runtime, ok := s.dynamicCandidateResolver.(ImportedBillingRuntime)
	if !ok {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	live, err := runtime.ReadImportedPurchaseBillingTerms(ctx, []ImportedBillingInput{{VersionNo: version, CurrencyCode: currency, Values: values}})
	if err != nil || len(live) != 1 {
		return nil, bizerr.WrapCode(err, CodePublicQuoteNotAvailable)
	}
	return s.buildNativeConfigurationQuote(ctx, version, revision, currency, source, contract, live[0])
}

func (s *publicCatalogReaderImpl) buildNativeConfigurationQuote(ctx context.Context, version, revision, currency string, source *publicQuoteSource, contract nativeImportPricingContract, live *channelcommercecap.BillingTerms) (*PublicConfigurationQuote, error) {
	if source == nil || len(source.TaxPolicies) != 1 {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	quote := &PublicConfigurationQuote{Revision: revision, CurrencyCode: currency, Tax: publicTaxSnapshot(taxDefinitionFromEntity(source.TaxPolicies[0])), Terms: []PublicQuoteTerm{}}
	markup, err := s.importedNativeMarkups(ctx, []string{version}, currency)
	if err != nil {
		return nil, err
	}
	for _, rule := range source.BillingRules {
		def := BillingRuleDefinitionFromEntity(rule)
		if def.PricePhase == PricePhaseRenewal {
			continue
		}
		token := markup[importWholePriceKey(version, rule.BillingRuleNo, currency)]
		term, err := nativeBillingTerm(contract.Snapshot, live, currency, token.reference, def)
		if err != nil {
			return nil, err
		}
		charge, setup, err := nativeBillingAmounts(term, token.markup)
		if err != nil {
			return nil, err
		}
		setup, err = nativePolicySetup(token, setup)
		if err != nil {
			return nil, err
		}
		total, err := addBillingAmount(charge, setup)
		if err != nil {
			return nil, err
		}
		lines := []PublicQuoteLine{{Kind: PublicQuoteLineBase, Quantity: 1, ChargeAmountMinor: charge, SetupFeeMinor: setup, PricedAmountMinor: total}}
		net, tax, gross, err := calculatePublicQuoteTax(lines, taxDefinitionFromEntity(source.TaxPolicies[0]))
		if err != nil {
			return nil, err
		}
		quote.Terms = append(quote.Terms, PublicQuoteTerm{TermNo: rule.BillingRuleNo, Status: PublicQuoteFinal, ChargeType: def.ChargeType, PeriodUnit: def.PeriodUnit, PeriodCount: def.PeriodCount, Cycles: 1, MinimumPurchaseCycles: 1, Lines: lines, NetAmountMinor: &net, TaxAmountMinor: &tax, TotalAmountMinor: &gross})
	}
	if len(quote.Terms) == 0 {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	return quote, nil
}

type nativeMarkup struct {
	reference, markup string
	fixedSetup        bool
	savedSetup        *int64
	status            string
}

func (s *publicCatalogReaderImpl) importedNativeMarkups(ctx context.Context, versions []string, currency string) (map[string]nativeMarkup, error) {
	var rows []struct {
		VersionNo, BillingRuleNo, CurrencyCode, UpstreamTermReference, MarkupPercent, WholeItemRule, StatusCode string
		SetupFeeMinor                                                                                           *int64
	}
	if err := s.database.Model(do.CatalogImportTerm{}).Ctx(ctx).As("term").LeftJoin("plugin_idc_core_catalog_price price", "price.tenant_id=term.tenant_id AND price.version_no=term.version_no AND price.billing_rule_no=term.billing_rule_no AND price.currency_code=term.currency_code AND price.option_no IS NULL AND price.configuration_price_dimension_no IS NULL AND price.minimum_quantity=0 AND price.maximum_quantity IS NULL").Where("term.tenant_id", 0).Where("term.currency_code", currency).WhereIn("term.version_no", versions).Fields("term.version_no,term.billing_rule_no,term.currency_code,term.upstream_term_reference,term.markup_percent,term.whole_item_rule,price.setup_fee_minor,price.status_code").Limit(maximumPublicationPrices + 1).Scan(&rows); err != nil {
		return nil, err
	}
	if len(rows) > maximumPublicationPrices {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	result := map[string]nativeMarkup{}
	for _, row := range rows {
		var whole channelcommercecap.ImportWholeItemRule
		if row.WholeItemRule != "" && row.WholeItemRule != "null" {
			if json.Unmarshal([]byte(row.WholeItemRule), &whole) != nil || whole.Validate() != nil {
				return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
			}
		}
		result[importWholePriceKey(row.VersionNo, row.BillingRuleNo, row.CurrencyCode)] = nativeMarkup{reference: row.UpstreamTermReference, markup: row.MarkupPercent, fixedSetup: whole.Kind == "fixed", savedSetup: row.SetupFeeMinor, status: row.StatusCode}
	}
	return result, nil
}

func nativeBillingTerm(snapshot channelcommercecap.ImportSnapshot, live *channelcommercecap.BillingTerms, currency, reference string, definition BillingRuleDefinition) (channelcommercecap.BillingTerm, error) {
	if live == nil || live.Validate() != nil || live.CurrencyCode != currency || live.ConfigurationRevision != snapshot.ConfigurationRevision || reference == "" {
		return channelcommercecap.BillingTerm{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	var result *channelcommercecap.BillingTerm
	for _, term := range live.Terms {
		if term.BillingTermToken != reference {
			continue
		}
		if result != nil {
			return channelcommercecap.BillingTerm{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		copyTerm := term
		result = &copyTerm
	}
	if result == nil || result.Status != "available" || result.ChargeType != string(definition.ChargeType) || result.PeriodUnit != string(definition.PeriodUnit) || result.PeriodCount != definition.PeriodCount || result.MinimumPurchaseCycles != 1 {
		return channelcommercecap.BillingTerm{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	return *result, nil
}
func nativeBillingAmounts(term channelcommercecap.BillingTerm, markup string) (int64, int64, error) {
	charge, err := strconv.ParseInt(term.SubtotalAmount, 10, 64)
	if err != nil {
		return 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	setup, err := strconv.ParseInt(term.SetupAmount, 10, 64)
	if err != nil {
		return 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	tax, err := strconv.ParseInt(term.TaxAmount, 10, 64)
	if err != nil || tax != 0 {
		return 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	total, err := strconv.ParseInt(term.TotalAmount, 10, 64)
	if err != nil {
		return 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	sum, err := addBillingAmount(charge, setup)
	if err != nil || sum != total {
		return 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	if markup == "" {
		markup = "0"
	}
	charge, err = markupImportMinor(charge, markup)
	if err != nil {
		return 0, 0, err
	}
	setup, err = markupImportMinor(setup, markup)
	return charge, setup, err
}

// nativePurchaseQuoteRows replaces only admitted native rows with one live
// account price per complete selection. All SQL evidence is loaded in batches.
func (s *publicCatalogReaderImpl) nativePurchaseQuoteRows(ctx context.Context, input PurchaseQuoteInput, versions []string, rules []entity.CatalogBillingRule, prices []entity.CatalogPrice) (PurchaseQuoteInput, []entity.CatalogPrice, error) {
	contracts, err := s.loadNativeImportPricing(ctx, versions)
	if err != nil {
		return input, nil, err
	}
	if len(contracts) == 0 {
		return input, prices, nil
	}
	runtime, ok := s.dynamicCandidateResolver.(ImportedBillingRuntime)
	if !ok {
		return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	snapshotNos := []string{}
	snapshotSeen := map[string]bool{}
	for _, line := range input.Lines {
		if _, native := contracts[line.VersionNo]; native {
			if line.ConfigurationSnapshotNo == "" || line.PurchaseCycles != 1 {
				return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
			}
			if !snapshotSeen[line.ConfigurationSnapshotNo] {
				snapshotNos = append(snapshotNos, line.ConfigurationSnapshotNo)
				snapshotSeen[line.ConfigurationSnapshotNo] = true
			}
		}
	}
	var rows []struct{ SnapshotNo, VersionNo, TypedValueSnapshot, CustomerValueDigest string }
	if err = s.database.Model(do.CommerceConfigurationSnapshot{}).Ctx(ctx).Where(do.CommerceConfigurationSnapshot{TenantId: 0}).WhereIn("configuration_snapshot_no", snapshotNos).Fields("configuration_snapshot_no AS snapshot_no,version_no,typed_value_snapshot,customer_value_digest").Limit(len(snapshotNos) + 1).Scan(&rows); err != nil {
		return input, nil, err
	}
	if len(rows) != len(snapshotNos) {
		return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	valuesBySnapshot := map[string][]configurationcontract.ConfigurationValue{}
	versionsBySnapshot := map[string]string{}
	for _, row := range rows {
		contract, found := contracts[row.VersionNo]
		if !found {
			return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		values, err := configurationcontract.DecodeCompiledValues(contract.Snapshot.Schema, []byte(row.TypedValueSnapshot))
		if err != nil {
			return input, nil, err
		}
		compiled, err := configurationcontract.CompileResolved(contract.Snapshot.Schema, values, configurationcontract.SurfaceCustomer, authorizedDynamicMethods(contract.Snapshot.Schema), nil)
		if err != nil || frozenConfigurationDigest(compiled.ValuesJSON) != row.CustomerValueDigest {
			return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		valuesBySnapshot[row.SnapshotNo] = values
		versionsBySnapshot[row.SnapshotNo] = row.VersionNo
	}
	remote := []ImportedBillingInput{}
	lineIndexes := []int{}
	for index, line := range input.Lines {
		if _, found := contracts[line.VersionNo]; !found {
			continue
		}
		if versionsBySnapshot[line.ConfigurationSnapshotNo] != line.VersionNo {
			return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		remote = append(remote, ImportedBillingInput{VersionNo: line.VersionNo, CurrencyCode: input.CurrencyCode, Values: valuesBySnapshot[line.ConfigurationSnapshotNo]})
		lineIndexes = append(lineIndexes, index)
	}
	live, err := runtime.ReadImportedPurchaseBillingTerms(ctx, remote)
	if err != nil || len(live) != len(remote) {
		return input, nil, bizerr.WrapCode(err, CodePublicQuoteNotAvailable)
	}
	markups, err := s.importedNativeMarkups(ctx, versions, input.CurrencyCode)
	if err != nil {
		return input, nil, err
	}
	ruleByKey := map[string]entity.CatalogBillingRule{}
	for _, rule := range rules {
		ruleByKey[purchaseQuotePair(rule.VersionNo, rule.BillingRuleNo)] = rule
	}
	result := make([]entity.CatalogPrice, 0, len(prices)+len(remote))
	for _, price := range prices {
		if _, native := contracts[price.VersionNo]; !native {
			result = append(result, price)
		}
	}
	input.Lines = append([]PurchaseQuoteLineInput(nil), input.Lines...)
	// Per-line configurations may share a version/rule but have different prices;
	// build each native base charge separately after ordinary quote assembly.
	for index, lineIndex := range lineIndexes {
		line := input.Lines[lineIndex]
		rule, found := ruleByKey[purchaseQuotePair(line.VersionNo, line.BillingRuleNo)]
		if !found {
			return input, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		metadata := markups[importWholePriceKey(line.VersionNo, line.BillingRuleNo, input.CurrencyCode)]
		term, err := nativeBillingTerm(contracts[line.VersionNo].Snapshot, live[index], input.CurrencyCode, metadata.reference, BillingRuleDefinitionFromEntity(rule))
		if err != nil {
			return input, nil, err
		}
		charge, setup, err := nativeBillingAmounts(term, metadata.markup)
		if err != nil {
			return input, nil, err
		}
		setup, err = nativePolicySetup(metadata, setup)
		if err != nil {
			return input, nil, err
		}
		input.Lines[lineIndex].PriceSelections = nil
		input.Lines[lineIndex].nativePrice = &nativePurchasePrice{charge: charge, setup: setup, term: term}
	}
	return input, result, nil
}

type nativePurchasePrice struct {
	charge, setup int64
	term          channelcommercecap.BillingTerm
}

func nativePurchaseQuoteCharge(line PurchaseQuoteLineInput) (PurchaseQuoteCharge, error) {
	if line.nativePrice == nil || line.PurchaseCycles != 1 {
		return PurchaseQuoteCharge{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	charge, err := multiplyPurchaseQuoteAmount(line.nativePrice.charge, int64(line.Quantity))
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	setup, err := multiplyPurchaseQuoteAmount(line.nativePrice.setup, int64(line.Quantity))
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	total, err := addBillingAmount(charge, setup)
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	digest, err := purchaseQuoteDigest(struct {
		Term          channelcommercecap.BillingTerm
		Snapshot      string
		Quantity      int
		Charge, Setup int64
	}{line.nativePrice.term, line.ConfigurationSnapshotNo, line.Quantity, charge, setup})
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	return PurchaseQuoteCharge{KindCode: "base", Quantity: 1, ChargeAmountMinor: charge, SetupFeeMinor: setup, PricedAmountMinor: total, PriceSourceDigest: digest}, nil
}

func nativePolicySetup(policy nativeMarkup, source int64) (int64, error) {
	if policy.savedSetup != nil && policy.status != "active" {
		return 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	if policy.savedSetup != nil && policy.fixedSetup {
		return *policy.savedSetup, nil
	}
	return source, nil
}
