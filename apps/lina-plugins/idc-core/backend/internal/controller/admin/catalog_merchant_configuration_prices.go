// Merchant configuration and prices share the existing save/apply transaction.
// This request-scoped target adapter holds no cache or independently built DI
// graph; both configuration and price owners retain their authorization checks.
package admin

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type merchantConfigurationPriceSource struct {
	service productautomation.Service
	targets []productautomation.MerchantPriceTargetInput
}

func (source merchantConfigurationPriceSource) ResolvePriceTargets(ctx context.Context, versionNo string, tokens []string) (map[string]string, error) {
	if len(tokens) != len(source.targets) {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	for index, token := range tokens {
		if token != source.targets[index].Token {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
	}
	return source.service.ResolveMerchantPriceTargets(ctx, versionNo, source.targets)
}

func merchantConfigurationPriceRows(inputs []*v1.CatalogMerchantConfigurationPriceRow) ([]catalog.ConfigurationPriceBatchRow, []productautomation.MerchantPriceTargetInput, error) {
	if len(inputs) > 256 {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	var (
		rows    = make([]catalog.ConfigurationPriceBatchRow, 0, len(inputs))
		targets = make([]productautomation.MerchantPriceTargetInput, 0, len(inputs))
	)
	for index, input := range inputs {
		if input == nil {
			return nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		tiers, err := catalogPriceScheduleTiers(input.Tiers)
		if err != nil {
			return nil, nil, err
		}
		mode := catalog.ConfigurationPriceMode(input.PricingMode)
		if mode == "" {
			mode = catalog.ConfigurationPriceFixed
			if input.Quantity {
				mode = catalog.ConfigurationPricePerUnit
			}
		}
		token := "merchant-price-" + strconv.Itoa(index)
		targets = append(targets, productautomation.MerchantPriceTargetInput{Token: token, FieldKey: input.FieldKey, CandidateValue: input.CandidateValue, Quantity: input.Quantity, PricingMode: mode})
		rows = append(rows, catalog.ConfigurationPriceBatchRow{PricingMode: mode, TargetKind: catalog.PriceBatchTargetConfiguration, CandidateToken: token, PreviousPriceNo: input.PreviousPriceNo, Schedule: catalog.PriceScheduleCreateInput{BillingRuleNo: input.BillingRuleNo, CurrencyCode: input.CurrencyCode, Tiers: tiers, Active: input.Active == nil || *input.Active}})
	}
	return rows, targets, nil
}

func mapMerchantPriceTargets(inputs []productautomation.MerchantPriceTarget) []*v1.CatalogMerchantPriceTarget {
	result := make([]*v1.CatalogMerchantPriceTarget, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, &v1.CatalogMerchantPriceTarget{FieldKey: input.FieldKey, CandidateValue: input.CandidateValue, Quantity: input.Quantity, DimensionNo: input.DimensionNo, PricingMode: string(input.PricingMode)})
	}
	return result
}
