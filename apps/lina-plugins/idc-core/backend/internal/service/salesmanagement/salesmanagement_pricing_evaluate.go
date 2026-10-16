// This file resolves all quote lines in a fixed number of local reads and
// applies customer-specific prices before active agent-tier prices.
package salesmanagement

import (
	"context"
	"math/big"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	adjustmentFixed    = "fixed_price"
	adjustmentDiscount = "discount_basis_points"
)

var _ PricingService = (*serviceImpl)(nil)

func (s *serviceImpl) EvaluatePrices(ctx context.Context, input PriceEvaluationInput) ([]PriceEvaluation, error) {
	if input.CustomerNo == "" || len(input.Lines) > 100 || len(input.CurrencyCode) != 3 || input.Now.IsZero() {
		return nil, bizerr.NewCode(codeInvalid)
	}
	result := []PriceEvaluation{}
	if len(input.Lines) == 0 {
		return result, nil
	}
	products := make([]string, 0, len(input.Lines))
	for _, line := range input.Lines {
		products = append(products, line.ProductNo)
	}
	rules, err := s.loadCurrentPriceRules(ctx, input.CustomerNo, products, input.CurrencyCode, input.Now)
	if err != nil {
		return nil, err
	}
	var globalFallback *entity.SalesPurchasePrice
	selected := make(map[string]entity.SalesPurchasePrice, len(rules))
	for _, rule := range rules {
		if rule.StatusCode != statusActive {
			continue
		}
		if rule.ProductNo == "" {
			fallbackRule := rule
			globalFallback = &fallbackRule
			continue
		}
		key := priceScope(rule.ProductNo, rule.ChargeTypeCode, rule.PeriodUnitCode, rule.PeriodCount)
		prior, ok := selected[key]
		if !ok || priceRulePriority(rule) > priceRulePriority(prior) {
			selected[key] = rule
		}
	}
	for _, line := range input.Lines {
		rule, ok := selected[priceScope(line.ProductNo, line.ChargeTypeCode, line.PeriodUnitCode, line.PeriodCount)]
		if !ok {
			if globalFallback == nil {
				continue
			}
			rule = *globalFallback
		}
		amount, err := evaluatePrice(line, rule)
		if err != nil {
			return nil, err
		}
		result = append(result, PriceEvaluation{ReferenceNo: line.ReferenceNo, PriceNo: rule.PriceNo, ContentDigest: rule.ContentDigest, Version: rule.Version, OriginalAmountMinor: line.PeriodSubtotalMinor, TargetAmountMinor: amount})
	}
	return result, nil
}
func priceScope(product, charge, period string, count int) string {
	return product + "\x00" + charge + "\x00" + period + "\x00" + strconv.Itoa(count)
}
func evaluatePrice(line PriceLine, rule entity.SalesPurchasePrice) (int64, error) {
	if line.Quantity < 1 || line.PurchaseCycles < 1 || line.PeriodSubtotalMinor < 0 {
		return 0, bizerr.NewCode(codeInvalid)
	}
	amount := big.NewInt(line.PeriodSubtotalMinor)
	switch rule.AdjustmentTypeCode {
	case adjustmentFixed:
		if rule.FixedPriceMinor < 0 {
			return 0, bizerr.NewCode(codeInvalid)
		}
		amount.Mul(big.NewInt(rule.FixedPriceMinor), big.NewInt(int64(line.Quantity)))
		amount.Mul(amount, big.NewInt(int64(line.PurchaseCycles)))
	case adjustmentDiscount:
		if rule.DiscountBasisPoints < 0 || rule.DiscountBasisPoints > 10000 {
			return 0, bizerr.NewCode(codeInvalid)
		}
		amount.Mul(amount, big.NewInt(int64(10000-rule.DiscountBasisPoints)))
		amount.Div(amount, big.NewInt(10000))
	default:
		return 0, bizerr.NewCode(codeInvalid)
	}
	if !amount.IsInt64() {
		return 0, bizerr.NewCode(codeInvalid)
	}
	return amount.Int64(), nil
}

// loadCurrentPriceRules performs two bounded set reads, shared by quoting and import projection.
func (s *serviceImpl) loadCurrentPriceRules(ctx context.Context, customerNo string, products []string, currency string, now time.Time) ([]entity.SalesPurchasePrice, error) {
	var levels []struct {
		LevelNo string `orm:"level_no"`
	}
	err := s.database.Model(dao.SalesAgentQualification.Table()+" q").Ctx(ctx).InnerJoin(dao.SalesAgentLevel.Table()+" l", "l.tenant_id=q.tenant_id AND l.level_no=q.level_no").Where("q.tenant_id", 0).Where("q.customer_no", customerNo).Where("q.status_code", statusActive).Where("l.status_code", statusActive).WhereLTE("q.effective_at", now).Where("q.expires_at IS NULL OR q.expires_at > ?", now).Fields("q.level_no").OrderDesc("q.id").Limit(1).Scan(&levels)
	if err != nil {
		return nil, managementError(err)
	}
	query := s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Where(do.SalesPurchasePrice{TenantId: 0}).WhereIn("product_no", products)
	if currency != "" {
		query = query.Where("currency_code", currency)
	}
	if len(levels) > 0 {
		query = query.Where("customer_no=? OR level_no=?", customerNo, levels[0].LevelNo)
	} else {
		query = query.Where("customer_no", customerNo)
	}
	// Only current versions enter the result; an inactive latest version cannot
	// accidentally revive an older discounted price.
	query = query.Where("NOT EXISTS (SELECT 1 FROM plugin_idc_core_sales_purchase_price newer WHERE newer.price_no=plugin_idc_core_sales_purchase_price.price_no AND newer.version>plugin_idc_core_sales_purchase_price.version)")
	var rules []entity.SalesPurchasePrice
	if err = query.Scan(&rules); err != nil {
		return nil, managementError(err)
	}
	if len(levels) > 0 {
		fallback, err := s.currentLevelPrice(ctx, levels[0].LevelNo)
		if err != nil {
			return nil, managementError(err)
		}
		if fallback != nil && fallback.StatusCode == statusActive {
			// The universal fallback also applies to authoritative renewal lines
			// whose historical billing term is absent from today's catalog.
			rules = append(rules, entity.SalesPurchasePrice{PriceNo: fallback.PriceNo, LevelNo: levels[0].LevelNo, AdjustmentTypeCode: adjustmentDiscount, DiscountBasisPoints: 10000 - fallback.PercentageBasisPoints, StatusCode: statusActive, Version: fallback.Version, ContentDigest: fallback.ContentDigest})
			// Resolve real catalog currency-period combinations once for all requested
			// products. Both checkout and API import receive identical fallback rules.
			query := s.database.Model(dao.CatalogProduct.Table()+" p").Ctx(ctx).
				InnerJoin(dao.CatalogBillingRule.Table()+" b", "b.tenant_id=p.tenant_id AND b.version_no=p.current_version_no").
				InnerJoin(dao.CatalogPrice.Table()+" c", "c.tenant_id=b.tenant_id AND c.version_no=b.version_no AND c.billing_rule_no=b.billing_rule_no").
				Where("p.tenant_id", 0).WhereIn("p.product_no", products).Where("b.status_code", statusActive).Where("c.status_code", statusActive)
			if currency != "" {
				query = query.Where("c.currency_code", currency)
			}
			terms, err := query.Fields("DISTINCT p.product_no,c.currency_code,b.charge_type_code,b.period_unit_code,b.period_count").Limit(4000).All()
			if err != nil {
				return nil, managementError(err)
			}
			if len(terms) >= 4000 {
				return nil, bizerr.NewCode(codeInvalid)
			}
			for _, term := range terms {
				rules = append(rules, entity.SalesPurchasePrice{PriceNo: fallback.PriceNo, LevelNo: levels[0].LevelNo, ProductNo: term["product_no"].String(), CurrencyCode: term["currency_code"].String(), ChargeTypeCode: term["charge_type_code"].String(), PeriodUnitCode: term["period_unit_code"].String(), PeriodCount: term["period_count"].Int(), AdjustmentTypeCode: adjustmentDiscount, DiscountBasisPoints: 10000 - fallback.PercentageBasisPoints, StatusCode: statusActive, Version: fallback.Version, ContentDigest: fallback.ContentDigest})
			}
		}
	}
	return rules, nil
}

// priceRulePriority keeps customer, product-tier and tier-wide rules independent
// of database return order. AGP references belong exclusively to tier defaults.
func priceRulePriority(rule entity.SalesPurchasePrice) int {
	if rule.CustomerNo != "" {
		return 3
	}
	if strings.HasPrefix(rule.PriceNo, "AGP_") {
		return 1
	}
	return 2
}

// ResolvePriceRules applies identical customer-before-tier precedence without inventing a selected configuration.
func (s *serviceImpl) ResolvePriceRules(ctx context.Context, customerNo, productNo string, now time.Time) ([]ResolvedPriceRule, error) {
	if customerNo == "" || productNo == "" || now.IsZero() {
		return nil, bizerr.NewCode(codeInvalid)
	}
	rules, err := s.loadCurrentPriceRules(ctx, customerNo, []string{productNo}, "", now)
	if err != nil {
		return nil, err
	}
	selected := make(map[string]entity.SalesPurchasePrice, len(rules))
	for _, rule := range rules {
		if rule.StatusCode != statusActive || rule.ProductNo == "" {
			continue
		}
		key := rule.CurrencyCode + "\x00" + priceScope(rule.ProductNo, rule.ChargeTypeCode, rule.PeriodUnitCode, rule.PeriodCount)
		prior, ok := selected[key]
		if !ok || priceRulePriority(rule) > priceRulePriority(prior) {
			selected[key] = rule
		}
	}
	result := make([]ResolvedPriceRule, 0, len(selected))
	for _, rule := range selected {
		item := ResolvedPriceRule{CurrencyCode: rule.CurrencyCode, ChargeTypeCode: rule.ChargeTypeCode, PeriodUnitCode: rule.PeriodUnitCode, PeriodCount: rule.PeriodCount}
		switch rule.AdjustmentTypeCode {
		case adjustmentFixed:
			value := rule.FixedPriceMinor
			item.Kind = "fixed"
			item.FixedAmountMinor = &value
		case adjustmentDiscount:
			value := rule.DiscountBasisPoints
			item.Kind = "discount"
			item.DiscountBasisPoints = &value
		default:
			return nil, bizerr.NewCode(codeInvalid)
		}
		result = append(result, item)
	}
	return result, nil
}
