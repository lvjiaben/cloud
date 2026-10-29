// This file creates editable revisions of published products without changing
// the current sale pointer, frozen historical prices or customer-specific prices.

package catalog

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *managementServiceImpl) commitProductRevision(ctx context.Context, input ProductCopyInput) (*ProductDraft, error) {
	if !existingCatalogReferencePattern.MatchString(input.SourceProductNo) || !existingCatalogReferencePattern.MatchString(input.SourceVersionNo) || gdb.TXFromCtx(ctx, dao.CatalogProduct.Group()) == nil {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	actor, err := s.authorizer.AuthorizeProductUpdate(ctx, input.SourceProductNo)
	if err != nil || actor <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	for _, check := range []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizeProductQuery(ctx, input.SourceProductNo)
		},
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizeBillingWrite(ctx, input.SourceVersionNo)
		},
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizePriceWrite(ctx, input.SourceVersionNo)
		},
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizeTaxWrite(ctx, input.SourceVersionNo)
		},
	} {
		current, checkErr := check(ctx)
		if checkErr != nil || current != actor {
			return nil, bizerr.NewCode(CodeManagementNotAccessible)
		}
	}
	var product *entity.CatalogProduct
	if err = dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.SourceProductNo}).LockUpdate().Scan(&product); err != nil {
		return nil, err
	}
	if product == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if product.CurrentVersionNo == nil || *product.CurrentVersionNo != input.SourceVersionNo {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	var versions []entity.CatalogProductVersion
	if err = dao.CatalogProductVersion.Ctx(ctx).Where(do.CatalogProductVersion{TenantId: 0, ProductNo: product.ProductNo}).OrderDesc("version_number").Limit(1).LockUpdate().Scan(&versions); err != nil {
		return nil, err
	}
	if len(versions) != 1 || versions[0].VersionNo != input.SourceVersionNo || versions[0].StatusCode != productSaleVersionPublished {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	source := versions[0]
	var inventory *entity.CatalogProductVersionInventory
	if err = dao.CatalogProductVersionInventory.Ctx(ctx).Where(do.CatalogProductVersionInventory{VersionNo: source.VersionNo}).LockUpdate().Scan(&inventory); err != nil {
		return nil, err
	}
	if inventory == nil {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	number, err := newCatalogNumber(s.random, "ver")
	if err != nil {
		return nil, err
	}
	inserted, err := dao.CatalogProductVersion.Ctx(ctx).Data(do.CatalogProductVersion{TenantId: 0, VersionNo: number, ProductNo: product.ProductNo,
		VersionNumber: source.VersionNumber + 1, StatusCode: catalogStatusDraft, SourceCode: source.SourceCode, SourceVersion: source.SourceVersion,
		SourceFingerprint: source.SourceFingerprint, ChangeSummary: "merchant_revision",
		CreatedBy: actor}).Insert()
	if err = requireSingleCatalogInsert(inserted, err, "create product revision"); err != nil {
		return nil, err
	}
	// Revisions share one product stock budget; reservations span its versions.
	inserted, err = dao.CatalogProductVersionInventory.Ctx(ctx).Data(do.CatalogProductVersionInventory{VersionNo: number, TrackingEnabled: inventory.TrackingEnabled, SellableQuantity: inventory.SellableQuantity, Version: 1}).Insert()
	if err = requireSingleCatalogInsert(inserted, err, "create revision inventory"); err != nil {
		return nil, err
	}
	if err = s.copyProductCommercialGraph(ctx, source.VersionNo, number, actor); err != nil {
		return nil, err
	}
	if err = s.copyRevisionCosts(ctx, source.VersionNo, number); err != nil {
		return nil, err
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product.revise", IdempotencyKey: number,
		OperationCode: "catalog.product.revise", TargetType: "catalog_product", TargetNo: product.ProductNo, ResultCode: "succeeded", ReasonCode: "manual_revision", RiskCode: "medium", RequiredPermission: permissionCatalogProductUpdate,
		BeforeSnapshot: map[string]any{"version_no": source.VersionNo, "state": product.StatusCode}, AfterSnapshot: map[string]any{"draft_version_no": number, "current_version_no": source.VersionNo, "inventory_tracking": inventory.TrackingEnabled, "sellable_quantity": inventory.SellableQuantity}}); err != nil {
		return nil, err
	}
	return &ProductDraft{ProductNo: product.ProductNo, VersionNo: number, VersionNumber: source.VersionNumber + 1, CategoryNo: product.CategoryNo, Status: product.StatusCode, VersionStatus: catalogStatusDraft, Source: source.SourceCode}, nil
}

// copyRevisionCosts retains bounded costs on a same-product edit using one
// collection read and one batch insert; separate product copies exclude costs.
func (s *managementServiceImpl) copyRevisionCosts(ctx context.Context, source, target string) error {
	var costs []entity.CatalogProductVersionCost
	if err := dao.CatalogProductVersionCost.Ctx(ctx).Where(do.CatalogProductVersionCost{TenantId: 0, VersionNo: source}).OrderAsc("id").Limit(maximumProductVersionCosts + 1).Scan(&costs); err != nil {
		return err
	}
	if len(costs) > maximumProductVersionCosts {
		return bizerr.NewCode(CodeManagementConflict)
	}
	rows := make([]do.CatalogProductVersionCost, 0, len(costs))
	for _, cost := range costs {
		number, err := newManagedCatalogNumber(s.random, "cst")
		if err != nil {
			return err
		}
		rows = append(rows, do.CatalogProductVersionCost{TenantId: 0, CostNo: number, VersionNo: target, CurrencyCode: cost.CurrencyCode, PeriodUnitCode: cost.PeriodUnitCode, PeriodCount: cost.PeriodCount, UnitCostMinor: cost.UnitCostMinor, EffectiveFrom: cost.EffectiveFrom, EffectiveUntil: cost.EffectiveUntil, Version: 1})
	}
	if len(rows) == 0 {
		return nil
	}
	inserted, err := dao.CatalogProductVersionCost.Ctx(ctx).Data(rows).Batch(256).Insert()
	return requireCatalogBatchInsert(inserted, err, len(rows), "retain product costs while editing")
}
