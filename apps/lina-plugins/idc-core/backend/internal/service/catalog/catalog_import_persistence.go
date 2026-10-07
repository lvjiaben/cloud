// This file claims actor-scoped import requests and writes catalog-owned drafts,
// price schedules and immutable source evidence in the caller's owner transaction.

package catalog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *managementServiceImpl) LookupImport(ctx context.Context, input ImportInput) (*ImportResult, error) {
	actor, err := s.authorizer.AuthorizeProductCreate(ctx)
	if err != nil {
		return nil, err
	}
	if actor <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	digest, err := validateImportInput(input)
	if err != nil {
		return nil, err
	}
	return loadImportResult(ctx, actor, input, digest, false)
}

func loadImportResult(ctx context.Context, actor int, input ImportInput, digest string, lock bool) (*ImportResult, error) {
	model := dao.CatalogProductImport.Ctx(ctx).Where(do.CatalogProductImport{TenantId: idccap.DefaultTenantID, ActorUserId: actor, RequestKey: input.RequestKey})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.CatalogProductImport
	if err := model.Scan(&row); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if row == nil {
		return nil, nil
	}
	if row.RequestDigest != digest {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	if row.ProductNo == nil || row.VersionNo == nil {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	missing := []ImportMissingConfiguration{}
	if err := json.Unmarshal([]byte(row.MissingConfigurationSnapshot), &missing); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return &ImportResult{Item: ProductDraft{ProductNo: *row.ProductNo, VersionNo: *row.VersionNo, VersionNumber: 1, CategoryNo: input.CategoryNo,
		Status: catalogStatusDraft, VersionStatus: catalogStatusDraft, Source: catalogSourceManual}, MissingConfiguration: missing, PricingIncomplete: row.PricingIncomplete}, nil
}

func (s *managementServiceImpl) CommitImport(ctx context.Context, input ImportInput, source *resourceinterfaceprojectioncap.CatalogDetail, missing []ImportMissingConfiguration) (*ImportResult, bool, error) {
	actor, err := s.authorizer.AuthorizeProductCreate(ctx)
	if err != nil {
		return nil, false, err
	}
	if actor <= 0 {
		return nil, false, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if gdb.TXFromCtx(ctx, dao.CatalogProduct.Group()) == nil {
		return nil, false, bizerr.NewCode(CodeManagementInvalid)
	}
	digest, err := validateImportInput(input)
	if err != nil {
		return nil, false, err
	}
	encoded, prices, incomplete, err := prepareImportSource(input, source)
	if err != nil {
		return nil, false, err
	}
	if missing == nil {
		missing = []ImportMissingConfiguration{}
	}
	if len(missing) > 256 {
		return nil, false, bizerr.NewCode(CodeManagementInvalid)
	}
	missingJSON, err := json.Marshal(missing)
	if err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementInvalid)
	}
	claim, err := dao.CatalogProductImport.Ctx(ctx).Data(do.CatalogProductImport{
		TenantId: idccap.DefaultTenantID, ActorUserId: actor, RequestKey: input.RequestKey, RequestDigest: digest,
		InterfaceInstanceNo: input.InterfaceInstanceNo, UpstreamReference: input.Reference, SourceSnapshot: string(encoded),
		MissingConfigurationSnapshot: string(missingJSON), PricingIncomplete: incomplete,
	}).InsertIgnore()
	if err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	affected, err := claim.RowsAffected()
	if err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if affected == 0 {
		result, replayErr := loadImportResult(ctx, actor, input, digest, true)
		return result, false, replayErr
	}
	if affected != 1 {
		return nil, false, bizerr.NewCode(CodeManagementConflict)
	}
	productNo, err := newCatalogNumber(s.random, "prd")
	if err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	versionNo, err := newCatalogNumber(s.random, "ver")
	if err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	nameZhCN, nameEnUS := input.NameZhCN, input.NameEnUS
	if nameZhCN == "" {
		nameZhCN = source.Product.Name
	}
	if nameEnUS == "" {
		nameEnUS = source.Product.Name
	}
	// The same empty draft contract/inventory semantics as manual creation apply:
	// inherit the existing contract policy and create the required inventory row.
	// Neither this nor the presence of prices grants publication or sale approval.
	normalized, err := normalizeProductCreate(ProductCreateInput{CategoryNo: input.CategoryNo, PublicSlug: "import-" + productNo[4:], NameZhCN: nameZhCN, NameEnUS: nameEnUS,
		SummaryZhCN: source.Product.Description, SummaryEnUS: source.Product.Description})
	if err != nil {
		return nil, false, err
	}
	draft, err := s.createProductDraftInTransaction(ctx, productNo, versionNo, actor, normalized)
	if err != nil {
		return nil, false, err
	}
	if err = s.insertImportPrices(ctx, versionNo, prices); err != nil {
		return nil, false, err
	}
	updated, err := dao.CatalogProductImport.Ctx(ctx).Where(do.CatalogProductImport{TenantId: idccap.DefaultTenantID, ActorUserId: actor, RequestKey: input.RequestKey, RequestDigest: digest}).
		Data(do.CatalogProductImport{ProductNo: &productNo, VersionNo: &versionNo}).UpdateAndGetAffected()
	if err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if updated != 1 {
		return nil, false, bizerr.NewCode(CodeManagementConflict)
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product.import", IdempotencyKey: productNo,
		OperationCode: "catalog.product.import", TargetType: "catalog_product", TargetNo: productNo, ResultCode: "succeeded", ReasonCode: "staff_import", RiskCode: "medium",
		RequiredPermission: permissionCatalogProductCreate, BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{"version_no": versionNo, "interface_instance_no": input.InterfaceInstanceNo, "pricing_incomplete": incomplete, "missing_configuration_count": len(missing)},
	}); err != nil {
		return nil, false, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return &ImportResult{Item: *draft, MissingConfiguration: missing, PricingIncomplete: incomplete}, true, nil
}

// insertImportPrices batches all currencies, sharing one new-purchase billing
// rule per proven interval. It never creates renewal prices or tax assumptions.
func (s *managementServiceImpl) insertImportPrices(ctx context.Context, versionNo string, prices []importPrice) error {
	if len(prices) == 0 {
		return nil
	}
	var (
		rules      = make([]do.CatalogBillingRule, 0, len(prices))
		rows       = make([]do.CatalogPrice, 0, len(prices))
		byInterval = make(map[string]string)
	)
	for _, price := range prices {
		key := fmt.Sprintf("%s-%d", price.definition.PeriodUnit, price.definition.PeriodCount)
		billingNo := byInterval[key]
		if billingNo == "" {
			var err error
			billingNo, err = newManagedCatalogNumber(s.random, "bil")
			if err != nil {
				return bizerr.WrapCode(err, CodeManagementUnavailable)
			}
			byInterval[key] = billingNo
			rules = append(rules, do.CatalogBillingRule{TenantId: idccap.DefaultTenantID, BillingRuleNo: billingNo, VersionNo: versionNo, RuleCode: "import-" + key,
				ChargeTypeCode: price.definition.ChargeType, PeriodUnitCode: price.definition.PeriodUnit, PeriodCount: price.definition.PeriodCount, PricePhaseCode: PricePhaseNewPurchase,
				QuantityModeCode: QuantityModeFixed, TrialQuantity: 0, StatusCode: catalogStatusActive, MinimumPurchaseCycles: 1})
		}
		priceNo, err := newManagedCatalogNumber(s.random, "prc")
		if err != nil {
			return bizerr.WrapCode(err, CodeManagementUnavailable)
		}
		rows = append(rows, do.CatalogPrice{TenantId: idccap.DefaultTenantID, PriceNo: priceNo, VersionNo: versionNo, BillingRuleNo: billingNo, CurrencyCode: price.currencyCode,
			AmountMinor: price.amountMinor, SetupFeeMinor: price.setupFeeMinor, MinimumQuantity: 0, StatusCode: catalogStatusActive})
	}
	result, err := dao.CatalogBillingRule.Ctx(ctx).Data(rules).Insert()
	if err = requireCatalogBatchInsert(result, err, len(rules), "insert imported billing rules"); err != nil {
		return err
	}
	result, err = dao.CatalogPrice.Ctx(ctx).Data(rows).Insert()
	if err = requireCatalogBatchInsert(result, err, len(rows), "insert imported price schedules"); err != nil {
		return gerror.Wrap(err, "persist imported prices")
	}
	return nil
}
