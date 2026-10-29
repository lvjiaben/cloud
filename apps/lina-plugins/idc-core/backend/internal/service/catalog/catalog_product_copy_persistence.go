// This file copies bounded catalog graphs in an existing owner transaction.
// Identifiers are regenerated; source stock, publication and historical facts
// are never updated. Each collection uses one read and batched writes.

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

func (s *managementServiceImpl) CommitProductCopy(ctx context.Context, input ProductCopyInput) (*ProductDraft, error) {
	if input.Revision {
		return s.commitProductRevision(ctx, input)
	}
	actor, err := s.authorizeProductCopy(ctx, input)
	if err != nil {
		return nil, err
	}
	if gdb.TXFromCtx(ctx, dao.CatalogProduct.Group()) == nil {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	var product *entity.CatalogProduct
	if err = dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.SourceProductNo}).LockUpdate().Scan(&product); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if product == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	var version *entity.CatalogProductVersion
	if err = dao.CatalogProductVersion.Ctx(ctx).Where(do.CatalogProductVersion{TenantId: 0, ProductNo: product.ProductNo, VersionNo: input.SourceVersionNo}).LockUpdate().Scan(&version); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if version == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	var inventory *entity.CatalogProductVersionInventory
	if err = dao.CatalogProductVersionInventory.Ctx(ctx).Where(do.CatalogProductVersionInventory{VersionNo: version.VersionNo}).LockUpdate().Scan(&inventory); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if inventory == nil {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	normalized, err := normalizeProductCreate(ProductCreateInput{CategoryNo: product.CategoryNo, PublicSlug: input.PublicSlug, NameZhCN: input.NameZhCN, NameEnUS: input.NameEnUS, SummaryZhCN: product.SummaryZhCn, SummaryEnUS: product.SummaryEnUs, SortOrder: product.SortOrder, InventoryTrackingEnabled: inventory.TrackingEnabled, SellableQuantity: 0})
	if err != nil {
		return nil, err
	}
	var newProduct, newVersion string
	newProduct, err = newCatalogNumber(s.random, "prd")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	newVersion, err = newCatalogNumber(s.random, "ver")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	result, err := s.createProductDraftInTransaction(ctx, newProduct, newVersion, actor, normalized)
	if err != nil {
		return nil, err
	}
	updated, err := dao.CatalogProductVersion.Ctx(ctx).Where(do.CatalogProductVersion{VersionNo: newVersion, StatusCode: catalogStatusDraft}).Data(do.CatalogProductVersion{SourceCode: catalogSourceCopy, SourceVersion: version.VersionNo}).Update()
	if err = requireSingleCatalogInsert(updated, err, "mark copied product source"); err != nil {
		return nil, err
	}
	if err = s.copyProductCommercialGraph(ctx, version.VersionNo, newVersion, actor); err != nil {
		return nil, err
	}
	result.Source = catalogSourceCopy
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product-draft.copy", IdempotencyKey: newProduct, OperationCode: "catalog.product-draft.copy", TargetType: "catalog_product", TargetNo: newProduct, ResultCode: "succeeded", ReasonCode: "manual_copy", RiskCode: "medium", RequiredPermission: permissionCatalogProductCreate, BeforeSnapshot: map[string]any{"source_product_no": product.ProductNo, "source_version_no": version.VersionNo}, AfterSnapshot: map[string]any{"version_no": newVersion, "state": catalogStatusDraft, "sellable_quantity": 0}}); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return result, nil
}

func (s *managementServiceImpl) authorizeProductCopy(ctx context.Context, input ProductCopyInput) (int, error) {
	actor, err := s.authorizer.AuthorizeProductQuery(ctx, input.SourceProductNo)
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	for _, authorize := range []func(context.Context) (int, error){s.authorizer.AuthorizeProductCreate,
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizeBillingWrite(ctx, input.SourceVersionNo)
		},
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizePriceWrite(ctx, input.SourceVersionNo)
		},
		func(ctx context.Context) (int, error) {
			return s.authorizer.AuthorizeTaxWrite(ctx, input.SourceVersionNo)
		}} {
		current, checkErr := authorize(ctx)
		if checkErr != nil || current != actor {
			return 0, bizerr.NewCode(CodeManagementNotAccessible)
		}
	}
	return actor, nil
}

func (s *managementServiceImpl) copyProductCommercialGraph(ctx context.Context, source, target string, actor int) error {
	var rules []entity.CatalogBillingRule
	if err := dao.CatalogBillingRule.Ctx(ctx).Where(do.CatalogBillingRule{TenantId: 0, VersionNo: source}).OrderAsc("id").Limit(maximumProductDetailBillingRules + 1).Scan(&rules); err != nil {
		return err
	}
	if len(rules) > maximumProductDetailBillingRules {
		return bizerr.NewCode(CodeManagementConflict)
	}
	ruleNumbers := make(map[string]string, len(rules))
	billingRows := make([]do.CatalogBillingRule, 0, len(rules))
	for _, rule := range rules {
		no, err := newManagedCatalogNumber(s.random, "bil")
		if err != nil {
			return err
		}
		ruleNumbers[rule.BillingRuleNo] = no
		row := portableBillingRow(target, no, BillingRuleDefinitionFromEntity(rule))
		row.RuleCode = rule.RuleCode
		row.StatusCode = rule.StatusCode
		billingRows = append(billingRows, row)
	}
	if len(billingRows) > 0 {
		insert, err := dao.CatalogBillingRule.Ctx(ctx).Data(billingRows).Batch(256).Insert()
		if err = requireCatalogBatchInsert(insert, err, len(billingRows), "copy billing rules"); err != nil {
			return err
		}
	}
	dimensionNumbers, err := s.copyProductPriceDimensions(ctx, source, target)
	if err != nil {
		return err
	}
	if err = s.copyProductPrices(ctx, source, target, ruleNumbers, dimensionNumbers); err != nil {
		return err
	}
	if err = s.copyProductTaxPolicies(ctx, source, target); err != nil {
		return err
	}
	return s.copyProductImportAndPolicy(ctx, source, target, ruleNumbers, actor)
}
