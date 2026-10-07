// This file manages bounded product-version cost settings with permission
// checks, parent locking, and optimistic concurrency.
package catalog

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const maximumProductVersionCosts = 256

// ProductVersionCost is one effective unit-cost setting for a product version.
type ProductVersionCost struct {
	CostNo         string
	VersionNo      string
	CurrencyCode   string
	PeriodUnit     PeriodUnit
	PeriodCount    int
	UnitCostMinor  int64
	EffectiveFrom  time.Time
	EffectiveUntil *time.Time
	Version        int64
}

// ProductVersionCostCreateInput carries one new effective unit-cost setting.
type ProductVersionCostCreateInput struct {
	VersionNo      string
	CurrencyCode   string
	PeriodUnit     PeriodUnit
	PeriodCount    int
	UnitCostMinor  int64
	EffectiveFrom  *time.Time
	EffectiveUntil *time.Time
}

// ProductVersionCostUpdateInput updates one setting at an expected version.
type ProductVersionCostUpdateInput struct {
	ProductVersionCostCreateInput
	CostNo          string
	ExpectedVersion int64
}

// ListProductVersionCosts returns a bounded list after exact cost authorization.
func (s *managementServiceImpl) ListProductVersionCosts(ctx context.Context, versionNo string) ([]ProductVersionCost, error) {
	if _, err := s.authorizer.AuthorizeCostWrite(ctx, versionNo); err != nil {
		return nil, err
	}
	versionNo = strings.TrimSpace(versionNo)
	if !existingCatalogReferencePattern.MatchString(versionNo) {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	var version *entity.CatalogProductVersion
	if err := dao.CatalogProductVersion.Ctx(ctx).Fields(dao.CatalogProductVersion.Columns().VersionNo).
		Where(do.CatalogProductVersion{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Scan(&version); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if version == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	rows := make([]entity.CatalogProductVersionCost, 0)
	if err := dao.CatalogProductVersionCost.Ctx(ctx).
		Where(do.CatalogProductVersionCost{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).
		OrderAsc(dao.CatalogProductVersionCost.Columns().CurrencyCode).
		OrderAsc(dao.CatalogProductVersionCost.Columns().PeriodUnitCode).
		OrderAsc(dao.CatalogProductVersionCost.Columns().PeriodCount).
		OrderDesc(dao.CatalogProductVersionCost.Columns().EffectiveFrom).
		Limit(maximumProductVersionCosts + 1).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if len(rows) > maximumProductVersionCosts {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	result := make([]ProductVersionCost, len(rows))
	for index := range rows {
		result[index] = productVersionCostFromEntity(rows[index])
	}
	return result, nil
}

// CreateProductVersionCost appends one effective setting under the locked version.
func (s *managementServiceImpl) CreateProductVersionCost(ctx context.Context, input ProductVersionCostCreateInput) (*ProductVersionCost, error) {
	if _, err := s.authorizer.AuthorizeCostWrite(ctx, input.VersionNo); err != nil {
		return nil, err
	}
	normalized, err := normalizeProductVersionCost(input, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	costNo, err := newManagedCatalogNumber(s.random, "cst")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	var result *ProductVersionCost
	err = dao.CatalogProductVersion.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := lockProductVersionCostParent(ctx, normalized.VersionNo); err != nil {
			return err
		}
		count, err := dao.CatalogProductVersionCost.Ctx(ctx).Where(do.CatalogProductVersionCost{
			TenantId: idccap.DefaultTenantID, VersionNo: normalized.VersionNo,
		}).Count()
		if err != nil {
			return gerror.Wrap(err, "count product version costs")
		}
		if count >= maximumProductVersionCosts {
			return bizerr.NewCode(CodeManagementConflict)
		}
		insert, err := dao.CatalogProductVersionCost.Ctx(ctx).Data(do.CatalogProductVersionCost{
			TenantId: idccap.DefaultTenantID, CostNo: costNo, VersionNo: normalized.VersionNo,
			CurrencyCode: normalized.CurrencyCode, PeriodUnitCode: string(normalized.PeriodUnit),
			PeriodCount: normalized.PeriodCount, UnitCostMinor: normalized.UnitCostMinor,
			EffectiveFrom: &normalized.EffectiveFrom, EffectiveUntil: normalized.EffectiveUntil,
		}).Insert()
		if err = requireSingleCatalogInsert(insert, err, "insert product version cost"); err != nil {
			return err
		}
		value := normalized
		value.CostNo, value.Version = costNo, 1
		result = &value
		return nil
	})
	return managementResult(result, err)
}

// UpdateProductVersionCost replaces mutable values using optimistic concurrency.
func (s *managementServiceImpl) UpdateProductVersionCost(ctx context.Context, input ProductVersionCostUpdateInput) (*ProductVersionCost, error) {
	if _, err := s.authorizer.AuthorizeCostWrite(ctx, input.VersionNo); err != nil {
		return nil, err
	}
	normalized, err := normalizeProductVersionCost(input.ProductVersionCostCreateInput, time.Now().UTC())
	if err != nil || !catalogBusinessNumberPattern.MatchString(strings.TrimSpace(input.CostNo)) || input.ExpectedVersion < 1 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	costNo := strings.TrimSpace(input.CostNo)
	var result *ProductVersionCost
	err = dao.CatalogProductVersionCost.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := lockProductVersionCostParent(ctx, normalized.VersionNo); err != nil {
			return err
		}
		var current *entity.CatalogProductVersionCost
		if err := dao.CatalogProductVersionCost.Ctx(ctx).Where(do.CatalogProductVersionCost{
			TenantId: idccap.DefaultTenantID, CostNo: costNo, VersionNo: normalized.VersionNo,
		}).LockUpdate().Scan(&current); err != nil {
			return gerror.Wrap(err, "lock product version cost")
		}
		if current == nil || current.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeManagementConflict)
		}
		updatedAt := time.Now().UTC()
		update, err := dao.CatalogProductVersionCost.Ctx(ctx).Where(do.CatalogProductVersionCost{
			TenantId: idccap.DefaultTenantID, CostNo: costNo, VersionNo: normalized.VersionNo, Version: input.ExpectedVersion,
		}).Data(do.CatalogProductVersionCost{
			CurrencyCode:   normalized.CurrencyCode,
			PeriodUnitCode: string(normalized.PeriodUnit),
			PeriodCount:    normalized.PeriodCount,
			UnitCostMinor:  normalized.UnitCostMinor,
			EffectiveFrom:  &normalized.EffectiveFrom,
			EffectiveUntil: normalized.EffectiveUntil,
			Version:        input.ExpectedVersion + 1,
			UpdatedAt:      &updatedAt,
		}).Update()
		if err = requireSingleCatalogWrite(update, err, "update product version cost"); err != nil {
			return err
		}
		if normalized.EffectiveUntil == nil {
			// A nil DO pointer is omitted by GoFrame. Use the narrow column/value
			// form only to clear this optional end time to a real SQL NULL.
			columns := dao.CatalogProductVersionCost.Columns()
			cleared, clearErr := dao.CatalogProductVersionCost.Ctx(ctx).Where(do.CatalogProductVersionCost{
				TenantId: idccap.DefaultTenantID, CostNo: costNo, VersionNo: normalized.VersionNo, Version: input.ExpectedVersion + 1,
			}).Data(columns.EffectiveUntil, nil).Update()
			if err = requireSingleCatalogWrite(cleared, clearErr, "clear product version cost effective end"); err != nil {
				return err
			}
		}
		value := normalized
		value.CostNo, value.Version = costNo, input.ExpectedVersion+1
		result = &value
		return nil
	})
	return managementResult(result, err)
}

func normalizeProductVersionCost(input ProductVersionCostCreateInput, now time.Time) (ProductVersionCost, error) {
	start := now
	if input.EffectiveFrom != nil {
		start = input.EffectiveFrom.UTC()
	}
	var end *time.Time
	if input.EffectiveUntil != nil {
		value := input.EffectiveUntil.UTC()
		end = &value
	}
	result := ProductVersionCost{
		VersionNo: strings.TrimSpace(input.VersionNo), CurrencyCode: strings.TrimSpace(input.CurrencyCode),
		PeriodUnit: input.PeriodUnit, PeriodCount: input.PeriodCount, UnitCostMinor: input.UnitCostMinor,
		EffectiveFrom: start, EffectiveUntil: end,
	}
	if !existingCatalogReferencePattern.MatchString(result.VersionNo) ||
		!currencyCodePattern.MatchString(result.CurrencyCode) ||
		(result.PeriodUnit != PeriodUnitOnce && result.PeriodUnit != PeriodUnitHour && result.PeriodUnit != PeriodUnitDay && result.PeriodUnit != PeriodUnitMonth && result.PeriodUnit != PeriodUnitYear) ||
		result.PeriodCount < 1 || result.PeriodCount > 87600 || result.UnitCostMinor < 0 ||
		(end != nil && !end.After(start)) {
		return ProductVersionCost{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return result, nil
}

func lockProductVersionCostParent(ctx context.Context, versionNo string) error {
	var version *entity.CatalogProductVersion
	if err := dao.CatalogProductVersion.Ctx(ctx).Fields(dao.CatalogProductVersion.Columns().VersionNo).
		Where(do.CatalogProductVersion{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).LockUpdate().Scan(&version); err != nil {
		return gerror.Wrap(err, "lock product version cost parent")
	}
	if version == nil {
		return bizerr.NewCode(CodeManagementNotAccessible)
	}
	return nil
}

func productVersionCostFromEntity(row entity.CatalogProductVersionCost) ProductVersionCost {
	result := ProductVersionCost{
		CostNo: row.CostNo, VersionNo: row.VersionNo, CurrencyCode: row.CurrencyCode,
		PeriodUnit: PeriodUnit(row.PeriodUnitCode), PeriodCount: row.PeriodCount,
		UnitCostMinor: row.UnitCostMinor, Version: row.Version,
	}
	if row.EffectiveFrom != nil {
		result.EffectiveFrom = row.EffectiveFrom.UTC()
	}
	if row.EffectiveUntil != nil {
		value := row.EffectiveUntil.UTC()
		result.EffectiveUntil = &value
	}
	return result
}
