// Configuration price tables commit as one bounded transaction. Related rules,
// dimensions and prior prices are loaded in sets, never once per visible row.
package catalog

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// SaveConfigurationPriceBatch checks staff price permission, locks one draft,
// fences all existing rows, validates every schedule, and commits all or none.
func (s *managementServiceImpl) SaveConfigurationPriceBatch(ctx context.Context, versionNo string, inputs []ConfigurationPriceBatchRow, targets ConfigurationPriceTargetSource) error {
	actor, err := s.authorizer.AuthorizePriceWrite(ctx, versionNo)
	if err != nil {
		return err
	}
	if actor <= 0 {
		return bizerr.NewCode(CodeManagementNotAccessible)
	}
	if !existingCatalogReferencePattern.MatchString(versionNo) || len(inputs) == 0 || len(inputs) > 256 {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	tokens, err := configurationBatchTargetTokens(inputs, targets)
	if err != nil {
		return err
	}
	err = dao.CatalogProductVersion.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, lockErr := lockEditableCatalogVersion(txCtx, versionNo); lockErr != nil {
			return lockErr
		}
		dimensions := map[string]string{}
		if len(tokens) > 0 {
			if err := requireLocalConfigurationPrices(txCtx, versionNo); err != nil {
				return err
			}
			var resolveErr error
			dimensions, resolveErr = targets.ResolvePriceTargets(txCtx, versionNo, tokens)
			if resolveErr != nil {
				return resolveErr
			}
		}
		var billingRules []*entity.CatalogBillingRule
		if readErr := dao.CatalogBillingRule.Ctx(txCtx).Where(do.CatalogBillingRule{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Limit(257).LockUpdate().Scan(&billingRules); readErr != nil {
			return readErr
		}
		if len(billingRules) > 256 {
			return bizerr.NewCode(CodeManagementConflict)
		}
		ruleByNo := map[string]*entity.CatalogBillingRule{}
		for _, rule := range billingRules {
			ruleByNo[rule.BillingRuleNo] = rule
		}
		var before []*entity.CatalogPrice
		if readErr := dao.CatalogPrice.Ctx(txCtx).Where(do.CatalogPrice{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Limit(maximumManagedPriceRows + 1).LockUpdate().Scan(&before); readErr != nil {
			return readErr
		}
		if len(before) > maximumManagedPriceRows {
			return bizerr.NewCode(CodeManagementConflict)
		}
		normalized, normalizeErr := normalizeConfigurationPriceBatch(versionNo, inputs, dimensions, ruleByNo, before)
		if normalizeErr != nil {
			return normalizeErr
		}

		var (
			deleting      = []string{}
			beforeChanged = []*entity.CatalogPrice{}
			inserts       = []catalogGenericPriceInsertRow{}
			changed       = map[string]bool{}
		)
		for _, item := range normalized {
			changed[configurationPriceScheduleIdentity(item.configurationPriceDimensionNo, item.billingRuleNo, item.currencyCode)] = true
		}
		for _, row := range before {
			if row.OptionNo == nil && changed[configurationPriceScheduleIdentity(row.ConfigurationPriceDimensionNo, row.BillingRuleNo, row.CurrencyCode)] {
				deleting = append(deleting, row.PriceNo)
				beforeChanged = append(beforeChanged, row)
			}
		}
		for _, item := range normalized {
			for _, tier := range item.tiers {
				number, numberErr := newManagedCatalogNumber(s.random, "prc")
				if numberErr != nil {
					return numberErr
				}
				inserts = append(inserts, catalogGenericPriceInsertRow{TenantId: idccap.DefaultTenantID, PriceNo: number, VersionNo: versionNo, BillingRuleNo: item.billingRuleNo, ConfigurationPriceDimensionNo: optionalStringPointer(item.configurationPriceDimensionNo), CurrencyCode: item.currencyCode, AmountMinor: tier.AmountMinor, SetupFeeMinor: tier.SetupFeeMinor, RenewalAmountMinor: tier.RenewalAmountMinor, MinimumQuantity: tier.MinimumQuantity, MaximumQuantity: tier.MaximumQuantity, StatusCode: item.status})
			}
		}
		if len(before)-len(deleting)+len(inserts) > maximumManagedPriceRows {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if len(deleting) > 0 {
			if _, deleteErr := dao.CatalogPrice.Ctx(txCtx).WhereIn("price_no", deleting).Where(do.CatalogPrice{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Delete(); deleteErr != nil {
				return deleteErr
			}
		}
		inserted, insertErr := dao.CatalogPrice.Ctx(txCtx).Data(inserts).Batch(256).InsertIgnore()
		if insertErr = requireCatalogBatchInsert(inserted, insertErr, len(inserts), "save configuration price table"); insertErr != nil {
			return insertErr
		}
		if err = saveNativeWholeItemPrices(txCtx, versionNo, normalized); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.configuration-price-table.save", IdempotencyKey: inserts[0].PriceNo, OperationCode: "catalog.configuration-price-table.save", TargetType: "catalog_product_version", TargetNo: versionNo, ResultCode: "succeeded", ReasonCode: "manual_save", RiskCode: "high", RequiredPermission: permissionCatalogPriceWrite, BeforeSnapshot: map[string]any{"prices": beforeChanged}, AfterSnapshot: map[string]any{"schedule_count": len(normalized), "prices": inserts}})
	})
	_, err = managementResult((*PriceSchedule)(nil), err)
	return err
}

func normalizeConfigurationPriceBatch(versionNo string, inputs []ConfigurationPriceBatchRow, dimensions map[string]string, rules map[string]*entity.CatalogBillingRule, before []*entity.CatalogPrice) ([]normalizedPriceScheduleCreate, error) {
	var (
		normalized = make([]normalizedPriceScheduleCreate, 0, len(inputs))
		seen       = map[string]bool{}
		current    = map[string][]*entity.CatalogPrice{}
	)
	for _, row := range before {
		if row.OptionNo == nil {
			key := configurationPriceScheduleIdentity(row.ConfigurationPriceDimensionNo, row.BillingRuleNo, row.CurrencyCode)
			current[key] = append(current[key], row)
		}
	}
	totalTiers := 0
	for _, input := range inputs {
		schedule := input.Schedule
		schedule.VersionNo = versionNo
		switch input.TargetKind {
		case PriceBatchTargetBase:
			if input.CandidateToken != "" {
				return nil, bizerr.NewCode(CodeManagementInvalid)
			}
			schedule.ConfigurationPriceDimensionNo = ""
		case "", PriceBatchTargetConfiguration:
			schedule.ConfigurationPriceDimensionNo = dimensions[input.CandidateToken]
			if input.CandidateToken == "" || schedule.ConfigurationPriceDimensionNo == "" {
				return nil, bizerr.NewCode(CodeManagementInvalid)
			}
		default:
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		item, err := normalizePriceScheduleCreate(schedule)
		if err != nil {
			return nil, err
		}
		key := configurationPriceScheduleIdentity(item.configurationPriceDimensionNo, item.billingRuleNo, item.currencyCode)
		if seen[key] {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		seen[key] = true
		rule := rules[item.billingRuleNo]
		mode := input.PricingMode
		if mode == "" {
			mode = ConfigurationPriceFixed
		}
		if input.TargetKind == PriceBatchTargetBase && mode != ConfigurationPriceFixed {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		if rule == nil || rule.StatusCode != catalogStatusActive || validateConfigurationPriceSchedule(BillingRuleDefinitionFromEntity(*rule), mode, item.tiers) != nil {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		existing := current[key]
		if (len(existing) == 0) != (input.PreviousPriceNo == "") {
			return nil, bizerr.NewCode(CodeManagementConflict)
		}
		if len(existing) > 0 {
			found := false
			for _, row := range existing {
				if row.PriceNo == input.PreviousPriceNo {
					found = true
				}
			}
			if !found {
				return nil, bizerr.NewCode(CodeManagementConflict)
			}
		}
		totalTiers += len(item.tiers)
		if totalTiers > 256 {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func configurationPriceScheduleIdentity(dimension, rule, currency string) string {
	return strings.Join([]string{dimension, rule, currency}, "\x00")
}

func configurationBatchTargetTokens(inputs []ConfigurationPriceBatchRow, targets ConfigurationPriceTargetSource) ([]string, error) {
	tokens := make([]string, 0, len(inputs))
	for _, input := range inputs {
		switch input.TargetKind {
		case "", PriceBatchTargetConfiguration:
			if input.CandidateToken == "" {
				return nil, bizerr.NewCode(CodeManagementInvalid)
			}
			tokens = append(tokens, input.CandidateToken)
		case PriceBatchTargetBase:
			if input.CandidateToken != "" {
				return nil, bizerr.NewCode(CodeManagementInvalid)
			}
		default:
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
	}
	if len(tokens) > 0 && targets == nil {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}

	return tokens, nil
}
