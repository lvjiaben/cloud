// This file manages immutable product-period price revisions. Mutation locks
// the customer or tier row so duplicate first versions cannot race.
package salesmanagement

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) ListPrices(ctx context.Context, input PageInput) (PricePage, error) {
	result := PricePage{Items: []PriceRule{}}
	if _, err := s.authorize(ctx, permissionPricing, input.Target != priceTargetLevel); err != nil {
		return result, err
	}
	if input.Target != "" && input.Target != priceTargetCustomer && input.Target != priceTargetLevel {
		return result, bizerr.NewCode(codeInvalid)
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	query := s.database.Model(dao.SalesPurchasePrice.Table()+" r").Ctx(ctx).
		LeftJoin(dao.CustomerSubject.Table()+" c", "c.tenant_id=r.tenant_id AND c.customer_no=r.customer_no").
		LeftJoin(dao.SalesAgentLevel.Table()+" l", "l.tenant_id=r.tenant_id AND l.level_no=r.level_no").
		LeftJoin(dao.CatalogProduct.Table()+" p", "p.tenant_id=r.tenant_id AND p.product_no=r.product_no").Where("r.tenant_id", 0).
		Where("NOT EXISTS (SELECT 1 FROM plugin_idc_core_sales_purchase_price newer WHERE newer.price_no=r.price_no AND newer.version>r.version)")
	query = query.WhereNot("r.status_code", statusDeleted)
	if input.ProductNo != "" {
		query = query.Where("r.product_no", input.ProductNo)
	}
	if input.CustomerNo != "" {
		query = query.Where("r.customer_no", input.CustomerNo)
	}
	if input.LevelNo != "" {
		query = query.Where("r.level_no", input.LevelNo)
	}
	if input.Target == priceTargetCustomer {
		query = query.WhereNotNull("r.customer_no")
	}
	if input.Target == priceTargetLevel {
		query = query.WhereNotNull("r.level_no")
	}
	if input.Search != "" {
		query = query.Where("p.name_zh_cn LIKE ? OR c.display_name LIKE ? OR l.name_zh_cn LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%", "%"+input.Search+"%")
	}
	result.Total, err = query.Clone().Count()
	if err != nil {
		return result, managementError(err)
	}
	rows, err := query.Fields("r.*,c.display_name AS customer_name,c.customer_code,l.name_zh_cn AS level_name,p.name_zh_cn AS product_name").OrderDesc("r.id").Page(input.Page, input.PageSize).All()
	if err != nil {
		return result, managementError(err)
	}
	for _, row := range rows {
		var model entity.SalesPurchasePrice
		if err = row.Struct(&model); err != nil {
			return result, managementError(err)
		}
		item := priceProjection(model)
		item.CustomerName = row["customer_name"].String()
		item.CustomerCode = row["customer_code"].String()
		item.LevelName = row["level_name"].String()
		item.ProductName = row["product_name"].String()
		result.Items = append(result.Items, item)
	}
	return result, nil
}
func (s *serviceImpl) SavePrice(ctx context.Context, input PriceInput) (PriceRule, error) {
	if _, err := s.authorize(ctx, permissionPricing, input.CustomerNo != ""); err != nil {
		return PriceRule{}, err
	}
	fixed, err := validatePriceInput(input)
	if err != nil {
		return PriceRule{}, err
	}
	var result PriceRule
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if input.CustomerNo != "" {
			exists, err := s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{TenantId: 0, CustomerNo: input.CustomerNo}).LockUpdate().One()
			if err != nil {
				return err
			}
			if len(exists) == 0 {
				return bizerr.NewCode(codeInvalid)
			}
		} else {
			exists, err := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0, LevelNo: input.LevelNo}).LockUpdate().One()
			if err != nil {
				return err
			}
			if len(exists) == 0 {
				return bizerr.NewCode(codeInvalid)
			}
		}
		var product *entity.CatalogProduct
		if err := s.database.Model(dao.CatalogProduct.Table()).Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.ProductNo}).Scan(&product); err != nil {
			return err
		}
		if product == nil || product.CurrentVersionNo == nil {
			return bizerr.NewCode(codeInvalid)
		}
		count, err := s.database.Model(dao.CatalogBillingRule.Table()+" b").Ctx(ctx).InnerJoin(dao.CatalogPrice.Table()+" p", "p.billing_rule_no=b.billing_rule_no AND p.version_no=b.version_no").Where("b.tenant_id", 0).Where("b.version_no", *product.CurrentVersionNo).Where("b.charge_type_code", input.ChargeTypeCode).Where("b.period_unit_code", input.PeriodUnitCode).Where("b.period_count", input.PeriodCount).Where("b.status_code", statusActive).Where("p.currency_code", input.CurrencyCode).Where("p.status_code", statusActive).Count()
		if err != nil {
			return err
		}
		if count == 0 {
			return bizerr.NewCode(codeInvalid)
		}
		query := s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Where(do.SalesPurchasePrice{TenantId: 0, ProductNo: input.ProductNo, CurrencyCode: input.CurrencyCode, ChargeTypeCode: input.ChargeTypeCode, PeriodUnitCode: input.PeriodUnitCode, PeriodCount: input.PeriodCount})
		if input.CustomerNo != "" {
			query = query.Where("customer_no", input.CustomerNo)
		} else {
			query = query.Where("level_no", input.LevelNo)
		}
		var previous *entity.SalesPurchasePrice
		if err = query.OrderDesc("version").Limit(1).Scan(&previous); err != nil {
			return err
		}
		version := int64(1)
		number := "APR_" + guid.S()
		if previous != nil {
			recreating := previous.StatusCode == statusDeleted && input.ExpectedVersion == 0 && input.PriceNo == ""
			if !recreating && (previous.StatusCode == statusDeleted || previous.Version != input.ExpectedVersion || previous.PriceNo != input.PriceNo) {
				return bizerr.NewCode(codeConflict)
			}
			version = previous.Version + 1
			number = previous.PriceNo
		} else if input.ExpectedVersion != 0 || input.PriceNo != "" {
			return bizerr.NewCode(codeConflict)
		}
		data := do.SalesPurchasePrice{TenantId: 0, PriceNo: number, ProductNo: input.ProductNo, CurrencyCode: input.CurrencyCode, ChargeTypeCode: input.ChargeTypeCode, PeriodUnitCode: input.PeriodUnitCode, PeriodCount: input.PeriodCount, AdjustmentTypeCode: input.AdjustmentType, FixedPriceMinor: fixed, DiscountBasisPoints: input.DiscountBasisPoints, StatusCode: input.Status, Version: version}
		if input.CustomerNo != "" {
			data.CustomerNo = input.CustomerNo
		} else {
			data.LevelNo = input.LevelNo
		}
		result = PriceRule{PriceNo: number, CustomerNo: input.CustomerNo, LevelNo: input.LevelNo, ProductNo: input.ProductNo, ProductName: product.NameZhCn, CurrencyCode: input.CurrencyCode, ChargeTypeCode: input.ChargeTypeCode, PeriodUnitCode: input.PeriodUnitCode, PeriodCount: input.PeriodCount, AdjustmentType: input.AdjustmentType, FixedPriceMinor: input.FixedPriceMinor, DiscountBasisPoints: input.DiscountBasisPoints, Status: input.Status, Version: version}
		digest, err := snapshotDigest(result)
		if err != nil {
			return err
		}
		data.ContentDigest = digest
		if _, err = s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Data(data).Insert(); err != nil {
			return err
		}
		return s.auditChange(ctx, permissionPricing, "price", number, input.CustomerNo, version, previous, result)
	})
	return result, managementError(err)
}

// DeletePrice serializes with saves on the same owner and appends a tombstone.
// No catalog availability checks are made: obsolete product prices remain removable.
func (s *serviceImpl) DeletePrice(ctx context.Context, priceNo string, expectedVersion int64) (PriceRule, error) {
	if _, err := s.authorize(ctx, permissionPricing, false); err != nil {
		return PriceRule{}, err
	}
	if priceNo == "" || len(priceNo) > 128 || expectedVersion < 1 {
		return PriceRule{}, bizerr.NewCode(codeInvalid)
	}
	var result PriceRule
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var owner *entity.SalesPurchasePrice
		if err := s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Where(do.SalesPurchasePrice{TenantId: 0, PriceNo: priceNo}).OrderDesc("version").Limit(1).Scan(&owner); err != nil {
			return err
		}
		if owner == nil {
			return bizerr.NewCode(codeConflict)
		}
		if _, err := s.authorize(ctx, permissionPricing, owner.CustomerNo != ""); err != nil {
			return err
		}
		var lock *gdb.Model
		if owner.CustomerNo != "" {
			lock = s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{TenantId: 0, CustomerNo: owner.CustomerNo})
		} else {
			lock = s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0, LevelNo: owner.LevelNo})
		}
		if _, err := lock.LockUpdate().One(); err != nil {
			return err
		}
		var previous *entity.SalesPurchasePrice
		if err := s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Where(do.SalesPurchasePrice{TenantId: 0, PriceNo: priceNo}).OrderDesc("version").Limit(1).Scan(&previous); err != nil {
			return err
		}
		if previous == nil || previous.Version != expectedVersion || previous.StatusCode == statusDeleted {
			return bizerr.NewCode(codeConflict)
		}
		result = priceProjection(*previous)
		result.Status = statusDeleted
		result.Version++
		digest, err := snapshotDigest(result)
		if err != nil {
			return err
		}
		data := do.SalesPurchasePrice{TenantId: 0, PriceNo: previous.PriceNo, ProductNo: previous.ProductNo, CurrencyCode: previous.CurrencyCode, ChargeTypeCode: previous.ChargeTypeCode, PeriodUnitCode: previous.PeriodUnitCode, PeriodCount: previous.PeriodCount, AdjustmentTypeCode: previous.AdjustmentTypeCode, StatusCode: statusDeleted, Version: result.Version, ContentDigest: digest}
		if previous.CustomerNo != "" {
			data.CustomerNo = previous.CustomerNo
		} else {
			data.LevelNo = previous.LevelNo
		}
		if previous.AdjustmentTypeCode == adjustmentFixed {
			data.FixedPriceMinor = previous.FixedPriceMinor
		} else {
			data.DiscountBasisPoints = previous.DiscountBasisPoints
		}
		if _, err = s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Data(data).Insert(); err != nil {
			return err
		}
		return s.auditChange(ctx, permissionPricing, "price", priceNo, previous.CustomerNo, result.Version, previous, result)
	})
	return result, managementError(err)
}

func priceProjection(row entity.SalesPurchasePrice) PriceRule {
	result := PriceRule{PriceNo: row.PriceNo, ProductNo: row.ProductNo, CurrencyCode: row.CurrencyCode, ChargeTypeCode: row.ChargeTypeCode, PeriodUnitCode: row.PeriodUnitCode, PeriodCount: row.PeriodCount, AdjustmentType: row.AdjustmentTypeCode, Status: row.StatusCode, Version: row.Version}
	result.CustomerNo = row.CustomerNo
	result.LevelNo = row.LevelNo
	if row.AdjustmentTypeCode == adjustmentFixed {
		result.FixedPriceMinor = strconv.FormatInt(row.FixedPriceMinor, 10)
	} else {
		value := row.DiscountBasisPoints
		result.DiscountBasisPoints = &value
	}
	return result
}

func validatePriceInput(input PriceInput) (*int64, error) {
	if (input.CustomerNo == "") == (input.LevelNo == "") || input.ProductNo == "" || len(input.CurrencyCode) != 3 || strings.ToUpper(input.CurrencyCode) != input.CurrencyCode || (input.Status != statusActive && input.Status != statusInactive) {
		return nil, bizerr.NewCode(codeInvalid)
	}
	var fixed *int64
	if input.AdjustmentType == adjustmentFixed {
		value, err := strconv.ParseInt(input.FixedPriceMinor, 10, 64)
		if err != nil || value < 0 || len(input.FixedPriceMinor) > 16 || input.DiscountBasisPoints != nil {
			return nil, bizerr.NewCode(codeInvalid)
		}
		fixed = &value
	} else if input.AdjustmentType != adjustmentDiscount || input.DiscountBasisPoints == nil || *input.DiscountBasisPoints < 0 || *input.DiscountBasisPoints > 10000 || input.FixedPriceMinor != "" {
		return nil, bizerr.NewCode(codeInvalid)
	}
	return fixed, nil
}
