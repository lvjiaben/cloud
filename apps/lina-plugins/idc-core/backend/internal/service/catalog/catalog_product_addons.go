// This file owns the bounded product-to-add-on offer set. Replacing the set
// never removes frozen quote/order target evidence or paid delivery results.

package catalog

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"strconv"
)

// ProductAddon is the minimum staff projection of a retained offer relation.
type ProductAddon struct{ ProductNo, NameZhCN, NameEnUS string }

// ProductAddonsInput uses the same optimistic product revision as sales copy.
type ProductAddonsInput struct {
	ProductNo  string
	ProductNos []string
	Version    int64
}

func (s *managementServiceImpl) GetProductAddons(ctx context.Context, productNo string) ([]ProductAddon, error) {
	if _, err := s.authorizer.AuthorizeProductQuery(ctx, productNo); err != nil {
		return nil, err
	}
	rows := []ProductAddon{}
	err := g.DB().GetScan(ctx, &rows, `SELECT offer.product_no,offer.name_zh_cn,offer.name_en_us FROM plugin_idc_core_catalog_product_addon link JOIN plugin_idc_core_catalog_product offer ON offer.tenant_id=link.tenant_id AND offer.product_no=link.addon_product_no WHERE link.tenant_id=0 AND link.product_no=? ORDER BY offer.sort_order,offer.product_no LIMIT 101`, productNo)
	if err != nil || len(rows) > 100 {
		return nil, bizerr.NewCode(CodeManagementUnavailable)
	}
	return rows, nil
}

func (s *managementServiceImpl) SetProductAddons(ctx context.Context, input ProductAddonsInput) (int64, error) {
	actor, err := s.authorizer.AuthorizeProductUpdate(ctx, input.ProductNo)
	if err != nil {
		return 0, err
	}
	if actor <= 0 {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if input.Version < 1 || len(input.ProductNos) > 100 || !existingCatalogReferencePattern.MatchString(input.ProductNo) {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	seen := make(map[string]bool, len(input.ProductNos))
	for _, no := range input.ProductNos {
		if no == input.ProductNo || !existingCatalogReferencePattern.MatchString(no) || seen[no] {
			return 0, bizerr.NewCode(CodeManagementInvalid)
		}
		seen[no] = true
	}
	raw, err := json.Marshal(input.ProductNos)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	err = dao.CatalogProduct.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var product *struct{ Version int64 }
		if err := dao.CatalogProduct.Ctx(ctx).Fields("version").Where(do.CatalogProduct{TenantId: 0, ProductNo: input.ProductNo}).LockUpdate().Scan(&product); err != nil {
			return err
		}
		if product == nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if product.Version != input.Version {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if len(input.ProductNos) > 0 {
			count, err := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0}).WhereIn("product_no", input.ProductNos).Count()
			if err != nil {
				return err
			}
			if count != len(input.ProductNos) {
				return bizerr.NewCode(CodeManagementInvalid)
			}
		}
		// Capture the actual previous offer set while the product revision is locked.
		var previous []struct{ AddonProductNo string }
		if err := g.DB().GetScan(ctx, &previous, `SELECT addon_product_no FROM plugin_idc_core_catalog_product_addon WHERE tenant_id=0 AND product_no=? ORDER BY addon_product_no`, input.ProductNo); err != nil {
			return err
		}
		before := make([]string, 0, len(previous))
		for _, offer := range previous {
			before = append(before, offer.AddonProductNo)
		}
		if _, err := g.DB().Exec(ctx, `DELETE FROM plugin_idc_core_catalog_product_addon WHERE tenant_id=0 AND product_no=?`, input.ProductNo); err != nil {
			return err
		}
		if len(input.ProductNos) > 0 {
			if _, err := g.DB().Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_addon(tenant_id,product_no,addon_product_no) SELECT 0,?,jsonb_array_elements_text(?::jsonb)`, input.ProductNo, raw); err != nil {
				return err
			}
		}
		result, err := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.ProductNo, Version: input.Version}).Data(do.CatalogProduct{Version: input.Version + 1, UpdatedBy: actor}).Update()
		if err = requireSingleCatalogWrite(result, err, "replace allowed add-on offers"); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product-addons.update", IdempotencyKey: input.ProductNo + ":" + strconv.FormatInt(input.Version, 10), OperationCode: "catalog.product-addons.update", TargetType: "catalog_product", TargetNo: input.ProductNo, ResultCode: "succeeded", ReasonCode: "manual_update", RiskCode: "medium", RequiredPermission: permissionCatalogProductUpdate, BeforeSnapshot: map[string]any{"product_nos": before, "version": product.Version}, AfterSnapshot: map[string]any{"product_nos": input.ProductNos, "version": input.Version + 1}})
	})
	if err != nil {
		_, wrapped := managementResult[int64](nil, err)
		return 0, wrapped
	}
	return input.Version + 1, nil
}
