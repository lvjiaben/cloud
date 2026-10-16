// This file remaps immutable configuration dimensions and copies exact money
// facts to a new draft. Writes are batched and historical identifiers stay put.

package catalog

import (
	"context"
	"encoding/hex"
	"io"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strings"
)

type copyPriceDimension struct {
	DimensionNo, FieldKey, CandidateValue, FieldLabelZhCN, FieldLabelEnUS, CandidateLabelZhCN, CandidateLabelEnUS, StatusCode string
	ConfigurationVersion                                                                                                      int64
	PricingMode                                                                                                               ConfigurationPriceMode `orm:"pricing_mode_code"`
}

func (s *managementServiceImpl) copyProductPriceDimensions(ctx context.Context, source, target string) (map[string]string, error) {
	var dimensions []copyPriceDimension
	db := dao.CatalogProduct.DB()
	if err := db.GetScan(ctx, &dimensions, `SELECT dimension_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,status_code,pricing_mode_code FROM plugin_idc_core_catalog_configuration_price_dimension WHERE tenant_id=0 AND version_no=? ORDER BY id LIMIT ?`, source, maximumConfigurationPriceDimensions+1); err != nil {
		return nil, err
	}
	if len(dimensions) > maximumConfigurationPriceDimensions {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	var (
		sqlRows []string
		args    []any
	)
	mapping := make(map[string]string, len(dimensions))
	for _, dimension := range dimensions {
		entropy := make([]byte, 16)
		if _, err := io.ReadFull(s.random, entropy); err != nil {
			return nil, err
		}
		no := "cpd_" + hex.EncodeToString(entropy)
		mapping[dimension.DimensionNo] = no
		sqlRows = append(sqlRows, "(0,?,?,?,?,?,?,?,?,?,?,?)")
		args = append(args, no, target, dimension.ConfigurationVersion, dimension.FieldKey, dimension.CandidateValue, dimension.FieldLabelZhCN, dimension.FieldLabelEnUS, dimension.CandidateLabelZhCN, dimension.CandidateLabelEnUS, dimension.StatusCode, dimension.PricingMode)
	}
	if len(sqlRows) > 0 {
		result, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,status_code,pricing_mode_code) VALUES `+strings.Join(sqlRows, ","), args...)
		if err = requireCatalogBatchInsert(result, err, len(sqlRows), "copy configuration price dimensions"); err != nil {
			return nil, err
		}
	}
	return mapping, nil
}

// copyCatalogPriceInsertRow keeps SQL NULL for absent optional references.
type copyCatalogPriceInsertRow struct {
	TenantId                                                    int `orm:"tenant_id"`
	PriceNo, VersionNo, BillingRuleNo, CurrencyCode, StatusCode string
	OptionNo                                                    *string `orm:"option_no"`
	ConfigurationPriceDimensionNo                               *string `orm:"configuration_price_dimension_no"`
	AmountMinor, SetupFeeMinor                                  int64
	RenewalAmountMinor                                          *int64
	MinimumQuantity                                             int64
	MaximumQuantity                                             *int64
}

func (s *managementServiceImpl) copyProductPrices(ctx context.Context, source, target string, rules, dimensions map[string]string) error {
	var prices []entity.CatalogPrice
	if err := dao.CatalogPrice.Ctx(ctx).Where(do.CatalogPrice{TenantId: 0, VersionNo: source}).OrderAsc("id").Limit(maximumProductDetailPrices + 1).Scan(&prices); err != nil {
		return err
	}
	if len(prices) > maximumProductDetailPrices {
		return bizerr.NewCode(CodeManagementConflict)
	}
	rows := make([]copyCatalogPriceInsertRow, 0, len(prices))
	for _, price := range prices {
		if price.OptionNo != nil {
			// Retired server option graphs cannot become new saleable catalog
			// models. Only plugin-declared configuration dimensions are copied.
			return bizerr.NewCode(CodeManagementConflict)
		}
		rule := rules[price.BillingRuleNo]
		dimension := dimensions[price.ConfigurationPriceDimensionNo]
		if rule == "" || price.ConfigurationPriceDimensionNo != "" && dimension == "" {
			return bizerr.NewCode(CodeManagementConflict)
		}
		no, err := newManagedCatalogNumber(s.random, "prc")
		if err != nil {
			return err
		}
		rows = append(rows, copyCatalogPriceInsertRow{TenantId: 0, PriceNo: no, VersionNo: target, BillingRuleNo: rule, ConfigurationPriceDimensionNo: optionalStringPointer(dimension), CurrencyCode: price.CurrencyCode, AmountMinor: price.AmountMinor, SetupFeeMinor: price.SetupFeeMinor, RenewalAmountMinor: price.RenewalAmountMinor, MinimumQuantity: price.MinimumQuantity, MaximumQuantity: price.MaximumQuantity, StatusCode: price.StatusCode})
	}
	if len(rows) == 0 {
		return nil
	}
	result, err := dao.CatalogPrice.Ctx(ctx).Data(rows).Batch(256).Insert()
	return requireCatalogBatchInsert(result, err, len(rows), "copy exact price schedules")
}

func (s *managementServiceImpl) copyProductTaxPolicies(ctx context.Context, source, target string) error {
	var taxes []entity.CatalogTaxPolicy
	if err := dao.CatalogTaxPolicy.Ctx(ctx).Where(do.CatalogTaxPolicy{TenantId: 0, VersionNo: source}).OrderAsc("id").Limit(maximumProductDetailTaxPolicies + 1).Scan(&taxes); err != nil {
		return err
	}
	if len(taxes) > maximumProductDetailTaxPolicies {
		return bizerr.NewCode(CodeManagementConflict)
	}
	taxRows := make([]do.CatalogTaxPolicy, 0, len(taxes))
	for _, tax := range taxes {
		no, err := newManagedCatalogNumber(s.random, "tax")
		if err != nil {
			return err
		}
		taxRows = append(taxRows, do.CatalogTaxPolicy{TenantId: 0, TaxPolicyNo: no, VersionNo: target, CurrencyCode: tax.CurrencyCode, PriceIncludesTax: tax.PriceIncludesTax, TaxTreatmentCode: tax.TaxTreatmentCode, TaxRateBasisPoints: tax.TaxRateBasisPoints, TaxCategoryCode: tax.TaxCategoryCode, RoundingScopeCode: tax.RoundingScopeCode, RoundingModeCode: tax.RoundingModeCode, RoundingQuantumMinor: tax.RoundingQuantumMinor})
	}
	if len(taxRows) > 0 {
		result, err := dao.CatalogTaxPolicy.Ctx(ctx).Data(taxRows).Insert()
		if err = requireCatalogBatchInsert(result, err, len(taxRows), "copy tax policies"); err != nil {
			return err
		}
	}
	return nil
}
