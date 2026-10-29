// This file appends validated billing rules and complete explicit-currency price schedules.

package catalog

import (
	"context"
	"database/sql"
	"encoding/hex"
	"io"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const maximumManagedPriceRows = 20_000

// BillingRuleCreateInput appends one billing model to an editable draft.
type BillingRuleCreateInput struct {
	VersionNo  string
	RuleCode   string
	Definition BillingRuleDefinition
	Active     bool
}

// BillingRule is the stable committed billing-rule projection.
type BillingRule struct {
	VersionNo     string
	BillingRuleNo string
	RuleCode      string
	Definition    BillingRuleDefinition
	Status        string
}

// PriceScheduleCreateInput atomically creates one complete base or explicit
// generic configuration-dimension schedule.
type PriceScheduleCreateInput struct {
	VersionNo                     string
	BillingRuleNo                 string
	ConfigurationPriceDimensionNo string
	CurrencyCode                  string
	Tiers                         []PriceTierDefinition
	Active                        bool
}

// PriceSchedule is the stable committed explicit-currency schedule projection.
type PriceSchedule struct {
	VersionNo                     string
	BillingRuleNo                 string
	ConfigurationPriceDimensionNo string
	CurrencyCode                  string
	PriceNumbers                  []string
	Tiers                         []PriceTierDefinition
	Status                        string
}

type normalizedBillingRuleCreate struct {
	versionNo  string
	ruleCode   string
	definition BillingRuleDefinition
	status     string
}

type normalizedPriceScheduleCreate struct {
	versionNo, billingRuleNo, configurationPriceDimensionNo, currencyCode, status string
	tiers                                                                         []PriceTierDefinition
}

func (s *managementServiceImpl) CreateBillingRule(
	ctx context.Context,
	input BillingRuleCreateInput,
) (*BillingRule, error) {
	actorUserID, err := s.authorizer.AuthorizeBillingWrite(ctx, input.VersionNo)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	normalized, err := normalizeBillingRuleCreate(input)
	if err != nil {
		return nil, err
	}
	billingRuleNo, err := newManagedCatalogNumber(s.random, "bil")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	var result *BillingRule
	err = dao.CatalogProductVersion.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := lockEditableCatalogVersion(ctx, normalized.versionNo); lockErr != nil {
			return lockErr
		}
		count, countErr := dao.CatalogBillingRule.Ctx(ctx).Where(do.CatalogBillingRule{
			TenantId: idccap.DefaultTenantID, VersionNo: normalized.versionNo,
		}).Count()
		if countErr != nil {
			return gerror.Wrap(countErr, "count catalog billing rules")
		}
		if count >= maximumPublicationBillingRules {
			return bizerr.NewCode(CodeManagementConflict)
		}
		definition := normalized.definition
		insert, insertErr := dao.CatalogBillingRule.Ctx(ctx).Data(do.CatalogBillingRule{
			TenantId: idccap.DefaultTenantID, BillingRuleNo: billingRuleNo,
			VersionNo: normalized.versionNo, RuleCode: normalized.ruleCode,
			ChargeTypeCode: definition.ChargeType, PeriodUnitCode: definition.PeriodUnit,
			PeriodCount: definition.PeriodCount, PricePhaseCode: definition.PricePhase,
			QuantityModeCode: definition.QuantityMode, TrialQuantity: definition.TrialQuantity,
			StatusCode: normalized.status, MinimumPurchaseCycles: definition.MinimumPurchaseCycles,
			MaximumRenewalCycles:    definition.MaximumRenewalCycles,
			MeterUnitCode:           optionalStringPointer(definition.MeterUnit),
			TierModeCode:            optionalStringPointer(definition.TierMode),
			UsageCollectionUnitCode: optionalStringPointer(string(definition.UsageCollectionUnit)),
			UsageCollectionCount:    definition.UsageCollectionCount,
			UsageRoundingModeCode:   optionalStringPointer(string(definition.UsageRoundingMode)),
			UsageRoundingQuantity:   definition.UsageRoundingQuantity,
			UsageMinimumQuantity:    definition.UsageMinimumQuantity,
			UsageMaximumQuantity:    definition.UsageMaximumQuantity,
			UsageMissingPolicyCode:  optionalStringPointer(string(definition.UsageMissingDataPolicy)),
		}).InsertIgnore()
		if insertErr = requireSingleCatalogInsert(insert, insertErr, "insert catalog billing rule"); insertErr != nil {
			return insertErr
		}
		result = &BillingRule{
			VersionNo: normalized.versionNo, BillingRuleNo: billingRuleNo,
			RuleCode: normalized.ruleCode, Definition: cloneBillingDefinition(definition),
			Status: normalized.status,
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.billing-rule.create", IdempotencyKey: billingRuleNo,
			OperationCode: "catalog.billing-rule.create", TargetType: "catalog_product_version",
			TargetNo: normalized.versionNo, ResultCode: "succeeded", ReasonCode: "manual_create",
			RequiredPermission: permissionCatalogBillingWrite,
			BeforeSnapshot:     map[string]any{},
			RiskCode:           "medium", AfterSnapshot: map[string]any{
				"state": normalized.status, "billing_rule_no": billingRuleNo,
				"charge_type": definition.ChargeType, "period_unit": definition.PeriodUnit,
				"period_count": definition.PeriodCount,
			},
		})
	})
	return managementResult(result, err)
}

func (s *managementServiceImpl) CreatePriceSchedule(
	ctx context.Context,
	input PriceScheduleCreateInput,
) (*PriceSchedule, error) {
	actorUserID, err := s.authorizer.AuthorizePriceWrite(ctx, input.VersionNo)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	normalized, err := normalizePriceScheduleCreate(input)
	if err != nil {
		return nil, err
	}
	priceNumbers := make([]string, len(normalized.tiers))
	for index := range priceNumbers {
		priceNumbers[index], err = newManagedCatalogNumber(s.random, "prc")
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
		}
	}
	var result *PriceSchedule
	err = dao.CatalogProductVersion.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := lockEditableCatalogVersion(ctx, normalized.versionNo); lockErr != nil {
			return lockErr
		}
		billingRule, lockErr := lockBillingRule(ctx, normalized.versionNo, normalized.billingRuleNo)
		if lockErr != nil {
			return lockErr
		}
		mode := ConfigurationPriceFixed
		if normalized.configurationPriceDimensionNo != "" {
			if err := requireLocalConfigurationPrices(ctx, normalized.versionNo); err != nil {
				return err
			}
			mode, lockErr = lockConfigurationPriceDimension(ctx, normalized.versionNo, normalized.configurationPriceDimensionNo)
			if lockErr != nil {
				return lockErr
			}
		}
		if validateErr := validateConfigurationPriceSchedule(
			BillingRuleDefinitionFromEntity(*billingRule), mode, normalized.tiers,
		); validateErr != nil {
			return bizerr.NewCode(CodeManagementInvalid)
		}
		if capacityErr := ensurePriceCapacity(ctx, normalized.versionNo, len(normalized.tiers)); capacityErr != nil {
			return capacityErr
		}
		if conflictErr := ensurePriceDimensionAvailable(ctx, normalized); conflictErr != nil {
			return conflictErr
		}
		rows := make([]catalogGenericPriceInsertRow, len(normalized.tiers))
		for index, tier := range normalized.tiers {
			rows[index] = catalogGenericPriceInsertRow{
				TenantId: idccap.DefaultTenantID, PriceNo: priceNumbers[index],
				VersionNo: normalized.versionNo, BillingRuleNo: normalized.billingRuleNo,
				ConfigurationPriceDimensionNo: optionalStringPointer(normalized.configurationPriceDimensionNo), CurrencyCode: normalized.currencyCode,
				AmountMinor: tier.AmountMinor, SetupFeeMinor: tier.SetupFeeMinor,
				RenewalAmountMinor: tier.RenewalAmountMinor, MinimumQuantity: tier.MinimumQuantity,
				MaximumQuantity: tier.MaximumQuantity, StatusCode: normalized.status,
			}
		}
		insert, insertErr := dao.CatalogPrice.Ctx(ctx).Data(rows).Batch(256).InsertIgnore()
		if insertErr = requireCatalogBatchInsert(insert, insertErr, len(rows), "insert catalog price schedule"); insertErr != nil {
			return insertErr
		}
		result = &PriceSchedule{
			VersionNo: normalized.versionNo, BillingRuleNo: normalized.billingRuleNo,
			ConfigurationPriceDimensionNo: normalized.configurationPriceDimensionNo, CurrencyCode: normalized.currencyCode,
			PriceNumbers: append([]string(nil), priceNumbers...),
			Tiers:        clonePriceTiers(normalized.tiers), Status: normalized.status,
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.price-schedule.create", IdempotencyKey: priceNumbers[0],
			OperationCode: "catalog.price-schedule.create", TargetType: "catalog_product_version",
			TargetNo: normalized.versionNo, ResultCode: "succeeded", ReasonCode: "manual_create",
			RequiredPermission: permissionCatalogPriceWrite,
			BeforeSnapshot:     map[string]any{},
			RiskCode:           "high", AfterSnapshot: map[string]any{
				"state": normalized.status, "billing_rule_no": normalized.billingRuleNo,
				"configuration_priced": normalized.configurationPriceDimensionNo != "", "currency_code": normalized.currencyCode,
				"tier_count": len(normalized.tiers),
			},
		})
	})
	return managementResult(result, err)
}

func normalizeBillingRuleCreate(input BillingRuleCreateInput) (normalizedBillingRuleCreate, error) {
	versionNo := strings.TrimSpace(input.VersionNo)
	ruleCode := strings.ToLower(strings.TrimSpace(input.RuleCode))
	definition := cloneBillingDefinition(input.Definition)
	definition.MeterUnit = strings.TrimSpace(definition.MeterUnit)
	definition.TierMode = strings.TrimSpace(definition.TierMode)
	status := "inactive"
	if input.Active {
		status = catalogStatusActive
	}
	if !existingCatalogReferencePattern.MatchString(versionNo) ||
		!componentCodePattern.MatchString(ruleCode) || ValidateBillingRule(definition) != nil {
		return normalizedBillingRuleCreate{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return normalizedBillingRuleCreate{
		versionNo: versionNo, ruleCode: ruleCode, definition: definition, status: status,
	}, nil
}

func normalizePriceScheduleCreate(input PriceScheduleCreateInput) (normalizedPriceScheduleCreate, error) {
	versionNo := strings.TrimSpace(input.VersionNo)
	billingRuleNo := strings.TrimSpace(input.BillingRuleNo)
	dimensionNo := strings.TrimSpace(input.ConfigurationPriceDimensionNo)
	status := "inactive"
	if input.Active {
		status = catalogStatusActive
	}
	if !existingCatalogReferencePattern.MatchString(versionNo) ||
		!catalogBusinessNumberPattern.MatchString(billingRuleNo) ||
		(dimensionNo != "" && !strings.HasPrefix(dimensionNo, "cpd_")) ||
		input.CurrencyCode != strings.TrimSpace(input.CurrencyCode) ||
		!currencyCodePattern.MatchString(input.CurrencyCode) || len(input.Tiers) == 0 ||
		len(input.Tiers) > maximumBillingTiers {
		return normalizedPriceScheduleCreate{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return normalizedPriceScheduleCreate{
		versionNo: versionNo, billingRuleNo: billingRuleNo, configurationPriceDimensionNo: dimensionNo,
		currencyCode: input.CurrencyCode, tiers: clonePriceTiers(input.Tiers), status: status,
	}, nil
}

func lockBillingRule(ctx context.Context, versionNo, billingRuleNo string) (*entity.CatalogBillingRule, error) {
	var rule *entity.CatalogBillingRule
	if err := dao.CatalogBillingRule.Ctx(ctx).Where(do.CatalogBillingRule{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo, BillingRuleNo: billingRuleNo,
	}).LockUpdate().Scan(&rule); err != nil {
		return nil, gerror.Wrap(err, "lock catalog billing rule")
	}
	if rule == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return rule, nil
}

func lockConfigurationPriceDimension(ctx context.Context, versionNo, dimensionNo string) (ConfigurationPriceMode, error) {
	var dimension *struct {
		DimensionNo string                 `orm:"dimension_no"`
		PricingMode ConfigurationPriceMode `orm:"pricing_mode_code"`
	}
	if err := dao.CatalogProductVersion.Ctx(ctx).Raw(`SELECT dimension_no,pricing_mode_code FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND dimension_no=? AND status_code='active' FOR UPDATE`, versionNo, dimensionNo).Scan(&dimension); err != nil {
		return "", gerror.Wrap(err, "lock configuration price dimension")
	}
	if dimension == nil {
		return "", bizerr.NewCode(CodeManagementNotAccessible)
	}
	return dimension.PricingMode, nil
}

func ensurePriceCapacity(ctx context.Context, versionNo string, adding int) error {
	count, err := dao.CatalogPrice.Ctx(ctx).Where(do.CatalogPrice{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
	}).Count()
	if err != nil {
		return gerror.Wrap(err, "count catalog prices")
	}
	if adding <= 0 || count > maximumManagedPriceRows-adding {
		return bizerr.NewCode(CodeManagementConflict)
	}
	return nil
}

func ensurePriceDimensionAvailable(ctx context.Context, input normalizedPriceScheduleCreate) error {
	model := dao.CatalogPrice.Ctx(ctx).Where(do.CatalogPrice{
		TenantId: idccap.DefaultTenantID, VersionNo: input.versionNo,
		BillingRuleNo: input.billingRuleNo, CurrencyCode: input.currencyCode,
	})
	if input.configurationPriceDimensionNo == "" {
		model = model.WhereNull(dao.CatalogPrice.Columns().OptionNo).WhereNull("configuration_price_dimension_no")
	} else {
		model = model.WhereNull(dao.CatalogPrice.Columns().OptionNo).Where("configuration_price_dimension_no", input.configurationPriceDimensionNo)
	}
	count, err := model.Count()
	if err != nil {
		return gerror.Wrap(err, "count catalog price dimension")
	}
	if count != 0 {
		return bizerr.NewCode(CodeManagementConflict)
	}
	return nil
}

type catalogGenericPriceInsertRow struct {
	TenantId                                                    int `orm:"tenant_id"`
	PriceNo, VersionNo, BillingRuleNo, CurrencyCode, StatusCode string
	ConfigurationPriceDimensionNo                               *string `orm:"configuration_price_dimension_no"`
	AmountMinor, SetupFeeMinor                                  int64
	RenewalAmountMinor                                          *int64
	MinimumQuantity                                             int64
	MaximumQuantity                                             *int64
}

func requireCatalogBatchInsert(result sql.Result, err error, expected int, operation string) error {
	if err != nil {
		return gerror.Wrap(err, operation)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read "+operation+" result")
	}
	if affected == 0 || affected != int64(expected) {
		return bizerr.NewCode(CodeManagementConflict)
	}
	return nil
}

func newManagedCatalogNumber(random io.Reader, prefix string) (string, error) {
	if random == nil || (prefix != "bil" && prefix != "bnd" && prefix != "cmp" && prefix != "cst" && prefix != "prc" && prefix != "rpu" && prefix != "tax") {
		return "", gerror.New("invalid managed catalog number dependency")
	}
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(random, randomBytes); err != nil {
		return "", gerror.Wrap(err, "read managed catalog number entropy")
	}
	result := prefix + "_" + hex.EncodeToString(randomBytes)
	if !catalogBusinessNumberPattern.MatchString(result) {
		return "", gerror.New("generated managed catalog number is invalid")
	}
	return result, nil
}

func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	result := value
	return &result
}

func cloneBillingDefinition(input BillingRuleDefinition) BillingRuleDefinition {
	result := input
	result.MaximumRenewalCycles = cloneIntPointer(input.MaximumRenewalCycles)
	result.UsageCollectionCount = cloneIntPointer(input.UsageCollectionCount)
	result.UsageRoundingQuantity = cloneInt64Pointer(input.UsageRoundingQuantity)
	result.UsageMinimumQuantity = cloneInt64Pointer(input.UsageMinimumQuantity)
	result.UsageMaximumQuantity = cloneInt64Pointer(input.UsageMaximumQuantity)
	return result
}

func clonePriceTiers(input []PriceTierDefinition) []PriceTierDefinition {
	result := make([]PriceTierDefinition, len(input))
	for index, tier := range input {
		result[index] = tier
		result[index].MaximumQuantity = cloneInt64Pointer(tier.MaximumQuantity)
		result[index].RenewalAmountMinor = cloneInt64Pointer(tier.RenewalAmountMinor)
	}
	return result
}
