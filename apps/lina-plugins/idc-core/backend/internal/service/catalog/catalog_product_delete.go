// This file removes catalog visibility while retaining immutable transactions,
// service access and renewal references. The write and audit commit together.
package catalog

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// ProductDeleteInput identifies the product and last-read metadata revision.
type ProductDeleteInput struct {
	ProductNo string
	Version   int64
}

// DeleteProduct soft-deletes sales metadata only; it never calls a provider.
func (s *managementServiceImpl) DeleteProduct(ctx context.Context, input ProductDeleteInput) error {
	actor, err := s.authorizer.AuthorizeProductDelete(ctx, input.ProductNo)
	if err != nil {
		return err
	}
	if actor <= 0 {
		return bizerr.NewCode(CodeManagementNotAccessible)
	}
	if input.Version < 1 || !existingCatalogReferencePattern.MatchString(input.ProductNo) {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	err = dao.CatalogProduct.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var previous *entity.CatalogProduct
		if err := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: idccap.DefaultTenantID, ProductNo: input.ProductNo}).LockUpdate().Scan(&previous); err != nil {
			return err
		}
		if previous == nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if previous.Version != input.Version {
			return bizerr.NewCode(CodeManagementConflict)
		}
		result, err := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: idccap.DefaultTenantID, ProductNo: input.ProductNo, Version: input.Version}).Data(do.CatalogProduct{StatusCode: "retired", UpdatedBy: actor, Version: input.Version + 1}).Update()
		if err = requireSingleCatalogWrite(result, err, "retire catalog product"); err != nil {
			return err
		}
		// Keep the metadata revision and framework soft deletion in one locked
		// transaction; GoFrame owns deleted_at and timestamp maintenance.
		result, err = dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: idccap.DefaultTenantID, ProductNo: input.ProductNo, Version: input.Version + 1}).Delete()
		if err = requireSingleCatalogWrite(result, err, "delete catalog product"); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product.delete", IdempotencyKey: input.ProductNo + ":" + strconv.FormatInt(input.Version, 10), OperationCode: "catalog.product.delete", TargetType: "catalog_product", TargetNo: input.ProductNo, ResultCode: "succeeded", ReasonCode: "merchant_delete", RiskCode: "high", RequiredPermission: permissionCatalogProductDelete, BeforeSnapshot: map[string]any{"status": previous.StatusCode, "version": previous.Version}, AfterSnapshot: map[string]any{"deleted": true, "status": "retired", "version": input.Version + 1}})
	})
	if err != nil {
		_, err = managementResult[int64](nil, err)
	}
	return err
}
