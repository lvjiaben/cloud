// This file previews one authorized saved editor version through the same
// configuration, billing, tax, and imported-whole-price calculations as checkout.
// A read-only repeatable snapshot prevents mixed configuration and price reads.

package catalog

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// ProductEditorPricePreviewInput carries customer choices and the current safe
// local candidate projection obtained by the existing configuration reader.
type ProductEditorPricePreviewInput struct {
	ProductNo, VersionNo, CurrencyCode string
	ConfigurationVersion               int64
	Values                             []configurationcontract.ConfigurationValue
	ResolvedCandidates                 configurationcontract.ResolvedCandidates
}

type ProductEditorPricePreview struct {
	VersionNo, CurrencyCode string
	PriceIncludesTax        bool
	Terms                   []ProductEditorPricePreviewTerm
}

type ProductEditorPricePreviewTerm struct {
	BillingRuleNo, Status                                       string
	ChargeType                                                  ChargeType
	PeriodUnit                                                  PeriodUnit
	PeriodCount, Cycles                                         int
	BaseAmountMinor, ConfigurationAmountMinor, SetupAmountMinor string
	TaxAmountMinor, TotalAmountMinor                            *string
	Issues                                                      []ProductEditorPricePreviewIssue
}

// ProductEditorPricePreviewIssue identifies only an explicit missing or invalid
// saved price schedule; authorization, tax and upstream errors remain errors.
type ProductEditorPricePreviewIssue struct {
	Kind                         CurrencyPriceDimensionKind
	Reason                       CurrencyPriceIssueReason
	FieldKey, NameZhCN, NameEnUS string
}

const productEditorPriceUnavailable = "unavailable"

func (s *productManagementReaderImpl) PreviewProductPrice(ctx context.Context, input ProductEditorPricePreviewInput) (*ProductEditorPricePreview, error) {
	if _, err := s.authorizer.AuthorizeProductQuery(ctx, input.ProductNo); err != nil {
		return nil, err
	}
	if !existingCatalogReferencePattern.MatchString(input.VersionNo) || !currencyCodePattern.MatchString(input.CurrencyCode) || len(input.Values) > 128 {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	for _, value := range input.Values {
		if len(value.FileTokens) > 0 {
			return nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
	}
	var result *ProductEditorPricePreview
	err := dao.CatalogProductVersion.Ctx(ctx).TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequired, ReadOnly: true, Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, _ gdb.TX) error {
		var err error
		result, err = loadProductEditorPricePreviewWithNative(ctx, input, s.importedBilling)
		return err
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	return result, nil
}

func loadProductEditorPricePreviewWithNative(ctx context.Context, input ProductEditorPricePreviewInput, billing resourceinterfaceprojectioncap.ImportedBillingSource) (*ProductEditorPricePreview, error) {
	var target publicStructuredConfigurationTarget
	err := dao.CatalogProductVersion.Ctx(ctx).As("version").
		LeftJoin("plugin_idc_core_catalog_product_version_configuration configuration", "configuration.tenant_id=version.tenant_id AND configuration.version_no=version.version_no").
		Fields("version.product_no,version.version_no,COALESCE(configuration.version,0) AS configuration_version,COALESCE(configuration.customer_declaration_snapshot,'{}') AS customer_declaration,COALESCE(configuration.customer_presentation_snapshot,'{}') AS customer_presentation").
		Where("version.tenant_id", 0).Where("version.product_no", input.ProductNo).Where("version.version_no", input.VersionNo).Scan(&target)
	if err != nil {
		return nil, err
	}
	if target.VersionNo == "" {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if target.ConfigurationVersion != input.ConfigurationVersion {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	schema, err := configurationcontract.DecodeDeclaration(target.CustomerDeclaration)
	if err != nil {
		return nil, err
	}
	evaluation, err := evaluateStructuredResolved(&publicStructuredConfigurationSnapshot{Target: target, Schema: schema}, input.Values, input.ResolvedCandidates)
	if err != nil || !evaluation.Valid {
		return nil, bizerr.WrapCode(err, CodePublicQuoteInvalid)
	}
	database := dao.CatalogProductVersion.DB()
	selections, _, err := loadSelectedConfigurationPriceDimensions(ctx, database, input.VersionNo, evaluation.Values, target.CustomerPresentation)
	if err != nil {
		return nil, err
	}
	source, err := loadPublicQuoteSource(ctx, database, input.VersionNo, input.CurrencyCode)
	if err != nil {
		return nil, err
	}
	reader := &publicCatalogReaderImpl{database: database}
	contracts, err := reader.loadNativeImportPricing(ctx, []string{input.VersionNo})
	if err != nil {
		return nil, err
	}
	var (
		quote  *PublicConfigurationQuote
		issues map[string][]CurrencyPriceIssue
	)
	if contract, native := contracts[input.VersionNo]; native {
		if billing == nil {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		mapped, mapErr := MapImportedBillingValues(&contract.Snapshot, evaluation.Values)
		if mapErr != nil {
			return nil, mapErr
		}
		live, readErr := billing.ReadImportedBillingTerms(ctx, resourceinterfaceprojectioncap.ImportedBillingRequest{Evidence: contract.Read, Input: channelcommercecap.ConfigurationInput{PublicSlug: contract.Snapshot.Product.PublicSlug, ConfigurationRevision: contract.Snapshot.ConfigurationRevision, CurrencyCode: input.CurrencyCode, Values: mapped}})
		if readErr != nil {
			return nil, readErr
		}
		quote, err = reader.buildNativeConfigurationQuote(ctx, input.VersionNo, "", input.CurrencyCode, source, contract, live)
	} else {
		quote, issues, err = buildProductEditorPriceQuote(input.CurrencyCode, source, configurationPriceSelectionMap(selections))
	}
	if err != nil {
		return nil, err
	}
	if err = applyImportedPublicPriceRules(ctx, database, input.VersionNo, quote); err != nil {
		return nil, err
	}
	result, err := productEditorPricePreview(input.VersionNo, quote)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		// One bounded projection serves every unavailable period; never query per row.
		var dimensions []configurationPriceDimensionIdentity
		if err = database.GetScan(ctx, &dimensions, `SELECT dimension_no,field_key,candidate_value FROM plugin_idc_core_catalog_configuration_price_dimension WHERE tenant_id=0 AND version_no=? AND status_code='active' LIMIT ?`, input.VersionNo, maximumConfigurationPriceDimensions+1); err != nil {
			return nil, err
		}
		if len(dimensions) > maximumConfigurationPriceDimensions {
			return nil, bizerr.NewCode(CodePublicQuoteUnavailable)
		}
		names := make(map[string]ProductEditorPricePreviewIssue, len(dimensions))
		for _, dimension := range dimensions {
			item := ProductEditorPricePreviewIssue{FieldKey: dimension.FieldKey}
			for _, field := range schema.Fields {
				if field.Key != dimension.FieldKey {
					continue
				}
				item.NameZhCN, item.NameEnUS = field.Label.ZhCN, field.Label.EnUS
				for _, candidate := range field.StaticCandidates {
					if candidate.Value == dimension.CandidateValue {
						item.NameZhCN += " / " + candidate.Label.ZhCN
						item.NameEnUS += " / " + candidate.Label.EnUS
						break
					}
				}
				break
			}
			names[dimension.DimensionNo] = item
		}
		for index := range result.Terms {
			for _, issue := range issues[result.Terms[index].BillingRuleNo] {
				item := names[issue.OptionNo]
				item.Kind, item.Reason = issue.Kind, issue.Reason
				result.Terms[index].Issues = append(result.Terms[index].Issues, item)
			}
		}
	}
	return result, nil
}

func productEditorPricePreview(version string, quote *PublicConfigurationQuote) (*ProductEditorPricePreview, error) {
	result := &ProductEditorPricePreview{VersionNo: version, CurrencyCode: quote.CurrencyCode, PriceIncludesTax: quote.Tax.PriceIncludesTax, Terms: make([]ProductEditorPricePreviewTerm, 0, len(quote.Terms))}
	for _, term := range quote.Terms {
		if term.Status == productEditorPriceUnavailable {
			result.Terms = append(result.Terms, ProductEditorPricePreviewTerm{BillingRuleNo: term.TermNo, Status: term.Status, ChargeType: term.ChargeType, PeriodUnit: term.PeriodUnit, PeriodCount: term.PeriodCount, Cycles: term.Cycles})
			continue
		}
		var base, options, setup int64
		for _, line := range term.Lines {
			target := &options
			if line.Kind == PublicQuoteLineBase {
				target = &base
			}
			var err error
			*target, err = addBillingAmount(*target, line.ChargeAmountMinor)
			if err != nil {
				return nil, err
			}
			setup, err = addBillingAmount(setup, line.SetupFeeMinor)
			if err != nil {
				return nil, err
			}
		}
		item := ProductEditorPricePreviewTerm{BillingRuleNo: term.TermNo, Status: term.Status, ChargeType: term.ChargeType, PeriodUnit: term.PeriodUnit, PeriodCount: term.PeriodCount, Cycles: term.Cycles, BaseAmountMinor: strconv.FormatInt(base, 10), ConfigurationAmountMinor: strconv.FormatInt(options, 10), SetupAmountMinor: strconv.FormatInt(setup, 10)}
		if term.TaxAmountMinor != nil {
			value := strconv.FormatInt(*term.TaxAmountMinor, 10)
			item.TaxAmountMinor = &value
		}
		if term.TotalAmountMinor != nil {
			value := strconv.FormatInt(*term.TotalAmountMinor, 10)
			item.TotalAmountMinor = &value
		}
		result.Terms = append(result.Terms, item)
	}
	return result, nil
}
