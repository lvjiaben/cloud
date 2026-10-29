// This file projects bounded publication readiness without changing catalog state.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Additional readiness dimensions owned by the publication boundary.
const (
	PublicationCheckLifecycle  PublicationCheckCode = "lifecycle"
	PublicationCheckCategory   PublicationCheckCode = "category"
	PublicationCheckAutomation PublicationCheckCode = "automation"
)

// PublicationReadinessReason is a safe, client-localized reason for a failed gate.
type PublicationReadinessReason string

const (
	PublicationReasonProviderUnavailable        PublicationReadinessReason = "provider_unavailable"
	PublicationReasonConfigurationPricesMissing PublicationReadinessReason = "configuration_prices_missing"
	PublicationReasonSupplierUnconfirmed        PublicationReadinessReason = "supplier_capabilities_unconfirmed"
	PublicationReasonConfigurationIncomplete    PublicationReadinessReason = "configuration_incomplete"
)

// PublicationAutomationReadiness reads the optional automation binding's
// authoritative new-business admission without exposing management details.
type PublicationAutomationReadiness interface {
	CheckPublicationReadiness(ctx context.Context, versionNo string) (PublicationReadinessCheck, error)
}

// PublicationPreview is a bounded, advisory projection. PublishVersion always rechecks.
type PublicationPreview struct {
	ProductNo string
	VersionNo string
	Ready     bool
	Checks    []PublicationReadinessCheck
}

// PublicationPreviewService reads readiness through the publication permission.
type PublicationPreviewService interface {
	PreviewVersion(ctx context.Context, versionNo string) (*PublicationPreview, error)
}

type publicationPreviewServiceImpl struct {
	authorizer PublicationAuthorizer
	automation PublicationAutomationReadiness
}

var _ PublicationPreviewService = (*publicationPreviewServiceImpl)(nil)

// NewPublicationPreview creates the read-only publication readiness service.
func NewPublicationPreview(
	authorizer PublicationAuthorizer,
	automation PublicationAutomationReadiness,
) (PublicationPreviewService, error) {
	if nilManagementDependency(authorizer) || nilManagementDependency(automation) {
		return nil, gerror.New("catalog publication preview requires authorization and automation readiness dependencies")
	}
	return &publicationPreviewServiceImpl{
		authorizer: authorizer, automation: automation,
	}, nil
}

// PreviewVersion authorizes before target access and returns stable check codes only.
func (s *publicationPreviewServiceImpl) PreviewVersion(
	ctx context.Context,
	versionNo string,
) (*PublicationPreview, error) {
	actorUserID, err := s.authorizer.AuthorizeProductPublication(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 || !existingCatalogReferencePattern.MatchString(versionNo) {
		return nil, bizerr.NewCode(CodePublicationInvalid)
	}
	version, product, err := loadCatalogPublicationPreviewTarget(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	checks := []PublicationReadinessCheck{{
		Code: PublicationCheckLifecycle,
		Passed: version.StatusCode == catalogStatusDraft && version.PublishedAt == nil &&
			version.RetiredAt == nil &&
			(product.StatusCode == catalogStatusDraft || product.StatusCode == catalogStatusActive),
	}}
	categoryValid, err := publicationCategoryValid(ctx, product)
	if err != nil {
		return nil, err
	}
	checks = append(checks, PublicationReadinessCheck{
		Code: PublicationCheckCategory, Passed: categoryValid,
	})
	completeness, err := evaluateCatalogPublicationCompleteness(ctx, product, version)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicationUnavailable)
	}
	checks = append(checks, completeness...)
	automationReady, err := s.publicationAutomationReady(ctx, version.VersionNo)
	if err != nil {
		return nil, err
	}
	checks = append(checks, PublicationReadinessCheck{
		Code: PublicationCheckAutomation, Passed: automationReady.Passed, ReasonCode: automationReady.ReasonCode,
	})
	preview := &PublicationPreview{
		ProductNo: product.ProductNo, VersionNo: version.VersionNo, Checks: checks, Ready: true,
	}
	for _, check := range checks {
		preview.Ready = preview.Ready && check.Passed
	}
	return preview, nil
}

func (s *publicationPreviewServiceImpl) publicationAutomationReady(
	ctx context.Context,
	versionNo string,
) (PublicationReadinessCheck, error) {
	check, err := s.automation.CheckPublicationReadiness(ctx, versionNo)
	check.Code = PublicationCheckAutomation
	if err == nil {
		return check, nil
	}
	check.Passed = false
	if bizerr.Is(err, CodePublicationNotAccessible) {
		return check, err
	}
	if bizerr.Is(err, CodeManagementConflict) || bizerr.Is(err, CodeManagementNotAccessible) {
		check.ReasonCode = PublicationReasonConfigurationIncomplete
		return check, nil
	}
	return check, bizerr.WrapCode(err, CodePublicationUnavailable)
}

func loadCatalogPublicationPreviewTarget(
	ctx context.Context,
	versionNo string,
) (*entity.CatalogProductVersion, *entity.CatalogProduct, error) {
	var version *entity.CatalogProductVersion
	versionColumns := dao.CatalogProductVersion.Columns()
	if err := dao.CatalogProductVersion.Ctx(ctx).Fields(
		versionColumns.VersionNo, versionColumns.ProductNo, versionColumns.StatusCode,
		versionColumns.SourceCode, versionColumns.PublishedAt, versionColumns.RetiredAt,
	).Where(do.CatalogProductVersion{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
	}).Scan(&version); err != nil {
		return nil, nil, gerror.Wrap(err, "load catalog publication preview version")
	}
	if version == nil {
		return nil, nil, bizerr.NewCode(CodePublicationNotAccessible)
	}
	var product *entity.CatalogProduct
	productColumns := dao.CatalogProduct.Columns()
	if err := dao.CatalogProduct.Ctx(ctx).Fields(
		productColumns.ProductNo, productColumns.CategoryNo, productColumns.StatusCode,
		productColumns.NameZhCn, productColumns.NameEnUs,
		productColumns.SummaryZhCn, productColumns.SummaryEnUs,
	).Where(do.CatalogProduct{
		TenantId: idccap.DefaultTenantID, ProductNo: version.ProductNo,
	}).Scan(&product); err != nil {
		return nil, nil, gerror.Wrap(err, "load catalog publication preview product")
	}
	if product == nil {
		return nil, nil, bizerr.NewCode(CodePublicationNotAccessible)
	}
	return version, product, nil
}

func publicationCategoryValid(ctx context.Context, product *entity.CatalogProduct) (bool, error) {
	var category *entity.CatalogCategory
	categoryColumns := dao.CatalogCategory.Columns()
	if err := dao.CatalogCategory.Ctx(ctx).Fields(categoryColumns.CategoryNo).
		Where(do.CatalogCategory{
			TenantId: idccap.DefaultTenantID, CategoryNo: product.CategoryNo,
			StatusCode: catalogStatusActive,
		}).Scan(&category); err != nil {
		return false, gerror.Wrap(err, "load catalog publication preview category")
	}
	return category != nil, nil
}
