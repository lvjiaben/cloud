// This file pauses and resumes product sales under the existing publication
// authority. It never cancels paid orders, mutates services or rewrites invoices.

package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strconv"
)

const (
	productSaleSuspended        = "suspended"
	productSaleVersionPublished = "published"
)

func (s *publicationServiceImpl) SetSaleState(ctx context.Context, input ProductSaleStateInput) (int64, error) {
	if input.ExpectedVersion < 1 || !existingCatalogReferencePattern.MatchString(input.ProductNo) || !existingCatalogReferencePattern.MatchString(input.VersionNo) {
		return 0, bizerr.NewCode(CodePublicationInvalid)
	}
	actor, err := s.authorizer.AuthorizeProductPublication(ctx, input.VersionNo)
	if err != nil {
		return 0, err
	}
	if actor <= 0 {
		return 0, bizerr.NewCode(CodePublicationNotAccessible)
	}
	var revision int64
	err = s.automation.WithSaleStateAdmission(ctx, input.VersionNo, input.Active, func(ctx context.Context) error {
		var product *entity.CatalogProduct
		if loadErr := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.ProductNo}).LockUpdate().Scan(&product); loadErr != nil {
			return loadErr
		}
		if product == nil {
			return bizerr.NewCode(CodePublicationNotAccessible)
		}
		if product.Version != input.ExpectedVersion || product.CurrentVersionNo == nil || *product.CurrentVersionNo != input.VersionNo || (product.StatusCode != catalogStatusActive && product.StatusCode != productSaleSuspended) {
			return bizerr.NewCode(CodePublicationState)
		}
		var version *entity.CatalogProductVersion
		if loadErr := dao.CatalogProductVersion.Ctx(ctx).Where(do.CatalogProductVersion{TenantId: 0, ProductNo: input.ProductNo, VersionNo: input.VersionNo}).LockUpdate().Scan(&version); loadErr != nil {
			return loadErr
		}
		if version == nil || version.StatusCode != productSaleVersionPublished {
			return bizerr.NewCode(CodePublicationState)
		}
		next := productSaleSuspended
		if input.Active {
			next = catalogStatusActive
			if _, checkErr := loadActiveCategoryForUpdate(ctx, product.CategoryNo); checkErr != nil {
				return checkErr
			}
			checks, checkErr := evaluateCatalogPublicationCompleteness(ctx, product, version)
			if checkErr != nil {
				return checkErr
			}
			if checkErr = saleReadinessError(checks); checkErr != nil {
				return checkErr
			}
			if checkErr := s.validatePublicationSource(ctx, version); checkErr != nil {
				return checkErr
			}
		}
		if product.StatusCode == next {
			revision = product.Version
			return nil
		}
		updated, writeErr := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.ProductNo, Version: input.ExpectedVersion}).Data(do.CatalogProduct{StatusCode: next, Version: input.ExpectedVersion + 1, UpdatedBy: actor}).Update()
		if writeErr = requireSingleCatalogWrite(updated, writeErr, "change product sale state"); writeErr != nil {
			return writeErr
		}
		revision = input.ExpectedVersion + 1
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product.sale-state", IdempotencyKey: product.ProductNo + ":" + strconv.FormatInt(input.ExpectedVersion, 10), OperationCode: "catalog.product.sale-state", TargetType: "catalog_product", TargetNo: product.ProductNo, ResultCode: "succeeded", ReasonCode: "manual_sale_state", RiskCode: "high", RequiredPermission: PermissionCatalogProductPublish, BeforeSnapshot: map[string]any{"state": product.StatusCode, "version": product.Version}, AfterSnapshot: map[string]any{"state": next, "version": revision, "current_version_no": input.VersionNo}})
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return 0, err
		}
		return 0, bizerr.WrapCode(err, CodePublicationUnavailable)
	}
	return revision, nil
}

// saleReadinessError reports the first actual missing business requirement.
// Publication and immediate-save internals keep their existing generic gate.
func saleReadinessError(checks []PublicationReadinessCheck) error {
	if len(checks) == 0 {
		return publicationIncomplete()
	}
	for _, check := range checks {
		if check.Passed {
			continue
		}
		switch check.Code {
		case PublicationCheckSalesCopy:
			return bizerr.NewCode(CodeSaleCopyRequired)
		case PublicationCheckPricing:
			return bizerr.NewCode(CodeSalePricingRequired)
		case PublicationCheckTax:
			return bizerr.NewCode(CodeSaleTaxRequired)
		default:
			return publicationIncomplete()
		}
	}
	return nil
}
