// This file copies only current agent-tier product prices to a new catalog
// draft. Customer-specific deals stay with the source product. History is not
// copied; new logical identities and digests are audited in the caller's TX.

package salesmanagement

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// copiedProductDraftState prevents copying prices into a published product.
const copiedProductDraftState = "draft"

// ProductPriceCopyService owns copying product-specific tier prices.
type ProductPriceCopyService interface {
	// CopyLevelProductPrices requires pricing permission and an owner TX; only
	// latest nondeleted tier prices copy, with customer prices excluded.
	CopyLevelProductPrices(context.Context, string, string) error
}

// ProductPriceCopyServiceFromService returns the existing startup instance.
func ProductPriceCopyServiceFromService(service Service) (ProductPriceCopyService, error) {
	result, ok := service.(ProductPriceCopyService)
	if !ok || result == nil {
		return nil, gerror.New("sales product-price copy service is unavailable")
	}
	return result, nil
}

func (s *serviceImpl) CopyLevelProductPrices(ctx context.Context, source, target string) error {
	if _, err := s.authorize(ctx, permissionPricing, false); err != nil {
		return err
	}
	if source == "" || source == target || gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return bizerr.NewCode(codeInvalid)
	}
	var products []entity.CatalogProduct
	if err := s.database.Model(do.CatalogProduct{}).Ctx(ctx).Where("tenant_id", 0).WhereIn("product_no", []string{source, target}).LockUpdate().Scan(&products); err != nil {
		return managementError(err)
	}
	if len(products) != 2 {
		return bizerr.NewCode(codeDenied)
	}
	targetName := ""
	for _, product := range products {
		if product.ProductNo == target {
			if product.StatusCode != copiedProductDraftState || product.CurrentVersionNo != nil {
				return bizerr.NewCode(codeConflict)
			}
			targetName = product.NameZhCn
		}
	}
	// The target product lock serializes concurrent copies; a second invocation
	// must not add a duplicate independent rule for the same commercial terms.
	countExisting, err := s.database.Model(do.SalesPurchasePrice{}).Ctx(ctx).Where(do.SalesPurchasePrice{TenantId: 0, ProductNo: target}).Count()
	if err != nil {
		return managementError(err)
	}
	if countExisting != 0 {
		return bizerr.NewCode(codeConflict)
	}
	var prices []entity.SalesPurchasePrice
	const query = `SELECT p.price_no,p.level_no,p.currency_code,p.charge_type_code,p.period_unit_code,p.period_count,p.adjustment_type_code,p.fixed_price_minor,p.discount_basis_points,p.status_code FROM plugin_idc_core_sales_purchase_price p
 WHERE p.tenant_id=0 AND p.product_no=? AND p.level_no IS NOT NULL AND p.customer_no IS NULL AND p.status_code<>'deleted'
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_sales_purchase_price n WHERE n.tenant_id=p.tenant_id AND n.price_no=p.price_no AND n.version>p.version)
 ORDER BY p.id LIMIT 4097 FOR UPDATE OF p`
	if err := s.database.GetScan(ctx, &prices, query, source); err != nil {
		return managementError(err)
	}
	if len(prices) > 4096 {
		return bizerr.NewCode(codeConflict)
	}
	rows := make([]do.SalesPurchasePrice, 0, len(prices))
	for _, price := range prices {
		no := "APR_" + guid.S()
		result := PriceRule{PriceNo: no, LevelNo: price.LevelNo, ProductNo: target, ProductName: targetName, CurrencyCode: price.CurrencyCode, ChargeTypeCode: price.ChargeTypeCode, PeriodUnitCode: price.PeriodUnitCode, PeriodCount: price.PeriodCount, AdjustmentType: price.AdjustmentTypeCode, Status: price.StatusCode, Version: 1}
		row := do.SalesPurchasePrice{TenantId: 0, PriceNo: no, LevelNo: price.LevelNo, ProductNo: target, CurrencyCode: price.CurrencyCode, ChargeTypeCode: price.ChargeTypeCode, PeriodUnitCode: price.PeriodUnitCode, PeriodCount: price.PeriodCount, AdjustmentTypeCode: price.AdjustmentTypeCode, StatusCode: price.StatusCode, Version: 1}
		if price.AdjustmentTypeCode == adjustmentFixed {
			result.FixedPriceMinor = strconv.FormatInt(price.FixedPriceMinor, 10)
			row.FixedPriceMinor = price.FixedPriceMinor
		} else {
			discount := price.DiscountBasisPoints
			result.DiscountBasisPoints = &discount
			row.DiscountBasisPoints = discount
		}
		digest, err := snapshotDigest(result)
		if err != nil {
			return managementError(err)
		}
		row.ContentDigest = digest
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	result, err := s.database.Model(dao.SalesPurchasePrice.Table()).Ctx(ctx).Data(rows).Batch(256).Insert()
	if err != nil {
		return managementError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return managementError(err)
	}
	if count != int64(len(rows)) {
		return bizerr.NewCode(codeConflict)
	}
	return managementError(s.auditChange(ctx, permissionPricing, "product_price_copy", target, "", 1, nil, struct {
		SourceProductNo  string
		CopiedPriceCount int
	}{source, len(rows)}))
}
