// This file persists complete portable schedules and their source contract in
// the owning import transaction. All dependent rows are batched; unknown prices
// never become zero and whole-item rules are stored separately from components.

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

type portableImportRows struct {
	rules         []do.CatalogBillingRule
	prices        []do.CatalogPrice
	taxes         []do.CatalogTaxPolicy
	terms         []do.CatalogImportTerm
	dimensionSQL  []string
	dimensionArgs []any
}

func (s *managementServiceImpl) insertPortableImport(ctx context.Context, versionNo string, input ImportInput, source *resourceinterfaceprojectioncap.CatalogDetail, prices []importPrice) error {
	if source.ReadEvidence == nil || source.ReadEvidence.Validate() != nil || source.ReadEvidence.InterfaceInstanceNo != input.InterfaceInstanceNo {
		return bizerr.NewCode(CodeManagementConflict)
	}
	rows, err := s.portableImportRows(versionNo, input.MarkupPercent, source.ImportSnapshot, prices)
	if err != nil {
		return err
	}
	if err = rows.persist(ctx); err != nil {
		return err
	}
	encoded, err := json.Marshal(source.ImportSnapshot)
	if err != nil {
		return bizerr.WrapCode(err, CodeManagementInvalid)
	}
	read, err := json.Marshal(source.ReadEvidence)
	if err != nil {
		return bizerr.WrapCode(err, CodeManagementInvalid)
	}
	digest := sha256.Sum256(encoded)
	result, err := dao.CatalogImportContract.Ctx(ctx).Data(do.CatalogImportContract{TenantId: 0, VersionNo: versionNo, InterfaceInstanceNo: input.InterfaceInstanceNo, Snapshot: string(encoded), SnapshotDigest: hex.EncodeToString(digest[:]), ReadEvidence: string(read)}).Insert()
	if err = requireSingleCatalogInsert(result, err, "freeze portable import contract"); err != nil {
		return err
	}
	return nil
}

func (s *managementServiceImpl) portableImportRows(versionNo, markup string, snapshot *channelcommercecap.ImportSnapshot, prices []importPrice) (*portableImportRows, error) {
	var (
		rows           = &portableImportRows{}
		rules          = map[string]string{}
		definitions    = map[string]BillingRuleDefinition{}
		taxes          = map[string]channelcommercecap.ImportTaxPolicy{}
		dimensions     = map[string]string{}
		dimensionModes = map[string]ConfigurationPriceMode{}
	)
	for _, price := range prices {
		p := price.portable
		if p == nil {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		definition := portableImportedBillingDefinition(snapshot, *p)
		if ValidateBillingRule(definition) != nil {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		if ValidateTaxPolicy(TaxPolicyDefinition{PriceIncludesTax: p.Tax.PriceIncludesTax, TaxTreatment: TaxTreatment(p.Tax.Treatment), TaxRateBasisPoints: p.Tax.RateBasisPoints, TaxCategoryCode: p.Tax.CategoryCode, RoundingScope: TaxRoundingScope(p.Tax.RoundingScope), RoundingMode: TaxRoundingMode(p.Tax.RoundingMode), RoundingQuantumMinor: p.Tax.RoundingQuantumMinor}) != nil {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		billingNo := rules[p.TermReference]
		if billingNo == "" {
			var err error
			billingNo, err = newManagedCatalogNumber(s.random, "bil")
			if err != nil {
				return nil, err
			}
			rules[p.TermReference], definitions[p.TermReference] = billingNo, definition
			rows.rules = append(rows.rules, portableBillingRow(versionNo, billingNo, definition))
		} else if !reflect.DeepEqual(definition, definitions[p.TermReference]) {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		if existing, ok := taxes[p.CurrencyCode]; ok {
			if !reflect.DeepEqual(existing, p.Tax) {
				return nil, bizerr.NewCode(CodeManagementInvalid)
			}
		} else {
			taxNo, err := newManagedCatalogNumber(s.random, "tax")
			if err != nil {
				return nil, err
			}
			taxes[p.CurrencyCode] = p.Tax
			rows.taxes = append(rows.taxes, do.CatalogTaxPolicy{TenantId: 0, TaxPolicyNo: taxNo, VersionNo: versionNo, CurrencyCode: p.CurrencyCode, PriceIncludesTax: p.Tax.PriceIncludesTax, TaxTreatmentCode: p.Tax.Treatment, TaxRateBasisPoints: p.Tax.RateBasisPoints, TaxCategoryCode: p.Tax.CategoryCode, RoundingScopeCode: p.Tax.RoundingScope, RoundingModeCode: p.Tax.RoundingMode, RoundingQuantumMinor: p.Tax.RoundingQuantumMinor})
		}
		if snapshot.PricingMode != channelcommercecap.ImportPricingRemoteEvaluation || len(p.Base) > 0 {
			if err := s.appendPortableBands(rows, versionNo, billingNo, p.CurrencyCode, "", ConfigurationPriceFixed, definition, p.Base); err != nil {
				return nil, err
			}
		}
		for _, option := range p.Options {
			candidateValue := option.Value
			mode := ConfigurationPriceMode(option.PricingMode)
			if mode == "" {
				mode = ConfigurationPriceFixed
			}
			if mode == ConfigurationPricePerUnit || mode == ConfigurationPriceBandTotal || mode == ConfigurationPriceGraduated {
				candidateValue = "quantity"
			}
			key := option.FieldKey + "\x00" + candidateValue
			dimensionNo := dimensions[key]
			if dimensionNo != "" && dimensionModes[key] != mode {
				return nil, bizerr.NewCode(CodeManagementInvalid)
			}
			if dimensionNo == "" {
				var err error
				entropy := make([]byte, 16)
				_, err = io.ReadFull(s.random, entropy)
				dimensionNo = "cpd_" + hex.EncodeToString(entropy)
				if err != nil {
					return nil, err
				}
				dimensions[key], dimensionModes[key] = dimensionNo, mode
				field := configurationFieldByKey(snapshot.Schema, option.FieldKey)
				if field == nil || field.Label == nil {
					return nil, bizerr.NewCode(CodeManagementInvalid)
				}
				labelZh, labelEn := field.Label.ZhCN, field.Label.EnUS
				for _, candidate := range field.StaticCandidates {
					if candidate != nil && candidate.Value == option.Value && candidate.Label != nil {
						labelZh, labelEn = candidate.Label.ZhCN, candidate.Label.EnUS
					}
				}
				rows.dimensionSQL = append(rows.dimensionSQL, "(0,?,?,1,?,?,?,?,?,?,?,'active')")
				rows.dimensionArgs = append(rows.dimensionArgs, dimensionNo, versionNo, option.FieldKey, candidateValue, field.Label.ZhCN, field.Label.EnUS, labelZh, labelEn, mode)
			}
			if err := s.appendPortableBands(rows, versionNo, billingNo, p.CurrencyCode, dimensionNo, ConfigurationPriceMode(option.PricingMode), definition, option.Bands); err != nil {
				return nil, err
			}
		}
		var whole any
		if p.WholeItemRule != nil {
			encoded, err := json.Marshal(p.WholeItemRule)
			if err != nil {
				return nil, err
			}
			whole = string(encoded)
		}
		if markup == "" {
			markup = "0"
		}
		rows.terms = append(rows.terms, do.CatalogImportTerm{TenantId: 0, VersionNo: versionNo, BillingRuleNo: billingNo, CurrencyCode: p.CurrencyCode, UpstreamTermReference: p.TermReference, WholeItemRule: whole, MarkupPercent: markup})
	}
	return rows, nil
}

func portableBillingDefinition(p channelcommercecap.ImportPriceSchedule) BillingRuleDefinition {
	result := BillingRuleDefinition{ChargeType: ChargeType(p.ChargeType), PeriodUnit: PeriodUnit(p.PeriodUnit), PeriodCount: p.PeriodCount, PricePhase: PricePhase(p.PricePhase), QuantityMode: QuantityMode(p.QuantityMode), TrialQuantity: p.TrialQuantity, MinimumPurchaseCycles: p.MinimumPurchaseCycles, MaximumRenewalCycles: cloneIntPointer(p.MaximumRenewalCycles)}
	if result.ChargeType == ChargeTypeTiered {
		result.TierMode = "graduated"
	}
	if p.Usage != nil {
		u := p.Usage
		result.MeterUnit = u.MeterUnit
		result.TierMode = u.TierMode
		result.UsageCollectionUnit = PeriodUnit(u.CollectionUnit)
		result.UsageCollectionCount = &u.CollectionCount
		result.UsageRoundingMode = RoundingMode(u.RoundingMode)
		result.UsageRoundingQuantity = &u.RoundingQuantity
		result.UsageMinimumQuantity = cloneInt64Pointer(u.MinimumQuantity)
		result.UsageMaximumQuantity = cloneInt64Pointer(u.MaximumQuantity)
		result.UsageMissingDataPolicy = MissingUsagePolicy(u.MissingPolicy)
	}
	return result
}

func portableBillingRow(versionNo, ruleNo string, d BillingRuleDefinition) do.CatalogBillingRule {
	row := do.CatalogBillingRule{TenantId: 0, VersionNo: versionNo, BillingRuleNo: ruleNo, RuleCode: "import-" + ruleNo[4:], ChargeTypeCode: d.ChargeType, PeriodUnitCode: d.PeriodUnit, PeriodCount: d.PeriodCount, PricePhaseCode: d.PricePhase, QuantityModeCode: d.QuantityMode, TrialQuantity: d.TrialQuantity, MinimumPurchaseCycles: d.MinimumPurchaseCycles, MaximumRenewalCycles: d.MaximumRenewalCycles, StatusCode: catalogStatusActive}
	if d.TierMode != "" {
		row.TierModeCode = &d.TierMode
	}
	if d.ChargeType == ChargeTypeUsage {
		unit, rounding, missing := string(d.UsageCollectionUnit), string(d.UsageRoundingMode), string(d.UsageMissingDataPolicy)
		row.MeterUnitCode = &d.MeterUnit
		row.UsageCollectionUnitCode = &unit
		row.UsageCollectionCount = d.UsageCollectionCount
		row.UsageRoundingModeCode = &rounding
		row.UsageRoundingQuantity = d.UsageRoundingQuantity
		row.UsageMinimumQuantity = d.UsageMinimumQuantity
		row.UsageMaximumQuantity = d.UsageMaximumQuantity
		row.UsageMissingPolicyCode = &missing
	}
	return row
}

func (s *managementServiceImpl) appendPortableBands(rows *portableImportRows, versionNo, ruleNo, currency, dimension string, mode ConfigurationPriceMode, definition BillingRuleDefinition, bands []channelcommercecap.ImportPriceBand) error {
	tiers := make([]PriceTierDefinition, len(bands))
	for i, band := range bands {
		tiers[i] = PriceTierDefinition{MinimumQuantity: band.MinimumQuantity, MaximumQuantity: band.MaximumQuantity, AmountMinor: band.AmountMinor, SetupFeeMinor: band.SetupAmountMinor, RenewalAmountMinor: band.RenewalAmountMinor}
	}
	if validateConfigurationPriceSchedule(definition, mode, tiers) != nil {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	for _, band := range bands {
		no, err := newManagedCatalogNumber(s.random, "prc")
		if err != nil {
			return err
		}
		var dimensionValue any
		if dimension != "" {
			dimensionValue = dimension
		}
		rows.prices = append(rows.prices, do.CatalogPrice{TenantId: 0, VersionNo: versionNo, PriceNo: no, BillingRuleNo: ruleNo, CurrencyCode: currency, ConfigurationPriceDimensionNo: dimensionValue, AmountMinor: band.AmountMinor, SetupFeeMinor: band.SetupAmountMinor, RenewalAmountMinor: band.RenewalAmountMinor, MinimumQuantity: band.MinimumQuantity, MaximumQuantity: band.MaximumQuantity, StatusCode: catalogStatusActive})
	}
	return nil
}

func (r *portableImportRows) persist(ctx context.Context) error {
	database := dao.CatalogProduct.DB()
	for _, batch := range []struct {
		table any
		rows  any
		count int
	}{{do.CatalogBillingRule{}, r.rules, len(r.rules)}, {do.CatalogTaxPolicy{}, r.taxes, len(r.taxes)}} {
		if batch.count == 0 {
			continue
		}
		result, err := database.Model(batch.table).Ctx(ctx).Data(batch.rows).Insert()
		if err = requireCatalogBatchInsert(result, err, batch.count, "insert portable catalog facts"); err != nil {
			return err
		}
	}
	if len(r.dimensionSQL) > 0 {
		result, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code) VALUES `+strings.Join(r.dimensionSQL, ","), r.dimensionArgs...)
		if err = requireCatalogBatchInsert(result, err, len(r.dimensionSQL), "insert portable configuration dimensions"); err != nil {
			return err
		}
	}
	for _, batch := range []struct {
		table any
		rows  any
		count int
	}{{do.CatalogPrice{}, r.prices, len(r.prices)}, {do.CatalogImportTerm{}, r.terms, len(r.terms)}} {
		if batch.count == 0 {
			continue
		}
		result, err := database.Model(batch.table).Ctx(ctx).Data(batch.rows).Insert()
		if err = requireCatalogBatchInsert(result, err, batch.count, "insert portable catalog prices"); err != nil {
			return err
		}
	}
	return nil
}

func portableImportedBillingDefinition(snapshot *channelcommercecap.ImportSnapshot, price channelcommercecap.ImportPriceSchedule) BillingRuleDefinition {
	definition := portableBillingDefinition(price)
	if snapshot.PricingMode == channelcommercecap.ImportPricingRemoteEvaluation && snapshot.Fulfillment.RenewalMode == "unavailable" {
		definition.PricePhase = PricePhaseNewPurchase
	}
	return definition
}
