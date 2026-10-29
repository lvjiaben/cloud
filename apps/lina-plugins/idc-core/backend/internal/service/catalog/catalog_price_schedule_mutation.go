// This file replaces or deletes complete draft price schedules, preserving
// published facts and using the previous row identity as a stale-edit fence.
package catalog

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// UpdatePriceSchedule replaces all tiers under one version lock. Changing the
// currency, rule or configuration target requires a separate explicit schedule.
func (s *managementServiceImpl) UpdatePriceSchedule(ctx context.Context, priceNo string, input PriceScheduleCreateInput) (*PriceSchedule, error) {
	var result *PriceSchedule
	err := s.mutatePriceSchedule(ctx, input.VersionNo, priceNo, func(txCtx context.Context, anchor *entity.CatalogPrice) error {
		if anchor.BillingRuleNo != input.BillingRuleNo || anchor.CurrencyCode != input.CurrencyCode || anchor.ConfigurationPriceDimensionNo != input.ConfigurationPriceDimensionNo {
			return bizerr.NewCode(CodeManagementConflict)
		}
		var err error
		result, err = s.CreatePriceSchedule(txCtx, input)
		return err
	}, "catalog.price-schedule.update")
	return managementResult(result, err)
}

// DeletePriceSchedule removes every tier of a draft schedule atomically.
func (s *managementServiceImpl) DeletePriceSchedule(ctx context.Context, versionNo, priceNo string) error {
	return s.mutatePriceSchedule(ctx, versionNo, priceNo, nil, "catalog.price-schedule.delete")
}

func (s *managementServiceImpl) mutatePriceSchedule(ctx context.Context, versionNo, priceNo string, replace func(context.Context, *entity.CatalogPrice) error, operation string) error {
	actor, err := s.authorizer.AuthorizePriceWrite(ctx, versionNo)
	if err != nil {
		return err
	}
	if actor <= 0 {
		return bizerr.NewCode(CodeManagementNotAccessible)
	}
	if !catalogBusinessNumberPattern.MatchString(priceNo) || !existingCatalogReferencePattern.MatchString(versionNo) {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	err = dao.CatalogProductVersion.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := lockEditableCatalogVersion(txCtx, versionNo); err != nil {
			return err
		}
		var anchor *entity.CatalogPrice
		if err := dao.CatalogPrice.Ctx(txCtx).Where(do.CatalogPrice{TenantId: idccap.DefaultTenantID, VersionNo: versionNo, PriceNo: priceNo}).LockUpdate().Scan(&anchor); err != nil {
			return gerror.Wrap(err, "read draft price schedule anchor")
		}
		if anchor == nil || anchor.OptionNo != nil {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if anchor.ConfigurationPriceDimensionNo != "" {
			if err := requireLocalConfigurationPrices(txCtx, versionNo); err != nil {
				return err
			}
		}
		query := dao.CatalogPrice.Ctx(txCtx).Where(do.CatalogPrice{TenantId: idccap.DefaultTenantID, VersionNo: versionNo, BillingRuleNo: anchor.BillingRuleNo, CurrencyCode: anchor.CurrencyCode}).WhereNull("option_no")
		if anchor.ConfigurationPriceDimensionNo == "" {
			query = query.WhereNull("configuration_price_dimension_no")
		} else {
			query = query.Where(do.CatalogPrice{ConfigurationPriceDimensionNo: anchor.ConfigurationPriceDimensionNo})
		}
		var before []*entity.CatalogPrice
		if err := query.Clone().OrderAsc("minimum_quantity").Limit(maximumBillingTiers + 1).Scan(&before); err != nil {
			return gerror.Wrap(err, "read draft price schedule")
		}
		if len(before) == 0 || len(before) > maximumBillingTiers {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if _, err := query.Delete(); err != nil {
			return gerror.Wrap(err, "remove draft price schedule")
		}
		if replace != nil {
			if err := replace(txCtx, anchor); err != nil {
				return err
			}
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: operation, IdempotencyKey: priceNo, OperationCode: operation, TargetType: "catalog_product_version", TargetNo: versionNo, ResultCode: "succeeded", ReasonCode: "manual_save", RequiredPermission: permissionCatalogPriceWrite, RiskCode: "high", BeforeSnapshot: map[string]any{"prices": before}, AfterSnapshot: map[string]any{"replaced": replace != nil}})
	})
	_, err = managementResult((*PriceSchedule)(nil), err)
	return err
}
