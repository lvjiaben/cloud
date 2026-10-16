// This file implements the sole catalog product-version publication transition.

package catalog

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// PublicationAuthorizer is the distinct staff publication authority.
type PublicationAuthorizer interface {
	AuthorizeProductPublication(ctx context.Context, versionNo string) (actorUserID int, err error)
}

// PublicationAutomationAdmission serializes publication with the optional
// automation binding and its interface lifecycle state. The action must run
// with the transaction context supplied by the gate.
type PublicationAutomationAdmission interface {
	WithPublicationAdmission(ctx context.Context, versionNo string, action func(context.Context) error) error
}

// PublicationResult is the committed version transition projection.
type PublicationResult struct {
	ProductNo           string
	VersionNo           string
	Status              string
	SupersededVersionNo string
	PublishedAt         time.Time
}

// PublicationService is the only service allowed to publish catalog versions.
type PublicationService interface {
	PublishVersion(ctx context.Context, versionNo string) (*PublicationResult, error)
}

type publicationServiceImpl struct {
	audit      idccap.BusinessAuditWriter
	authorizer PublicationAuthorizer
	automation PublicationAutomationAdmission
}

var _ PublicationService = (*publicationServiceImpl)(nil)

// NewPublication creates the unified merchant-managed catalog publisher.
func NewPublication(
	auditWriter idccap.BusinessAuditWriter,
	authorizer PublicationAuthorizer,
	automation PublicationAutomationAdmission,
) (PublicationService, error) {
	if nilManagementDependency(auditWriter) || nilManagementDependency(authorizer) || nilManagementDependency(automation) {
		return nil, gerror.New("catalog publication requires audit, authorization, and automation admission dependencies")
	}
	return &publicationServiceImpl{
		audit: auditWriter, authorizer: authorizer, automation: automation,
	}, nil
}

// PublishVersion authorizes before target access and commits the old version,
// target version, product pointer, and business audit in one transaction.
func (s *publicationServiceImpl) PublishVersion(
	ctx context.Context,
	versionNo string,
) (*PublicationResult, error) {
	actorUserID, err := s.authorizer.AuthorizeProductPublication(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 || !existingCatalogReferencePattern.MatchString(versionNo) {
		return nil, bizerr.NewCode(CodePublicationInvalid)
	}
	var result *PublicationResult
	err = s.automation.WithPublicationAdmission(ctx, versionNo, func(txCtx context.Context) error {
		result, err = s.publishVersionInTransaction(txCtx, versionNo, actorUserID)
		return err
	})
	if err == nil {
		return result, nil
	}
	if bizerr.Is(err, CodePublicationNotAccessible) || bizerr.Is(err, CodePublicationInvalid) ||
		bizerr.Is(err, CodePublicationState) || bizerr.Is(err, CodePublicationIncomplete) {
		return nil, err
	}
	if bizerr.Is(err, CodeManagementConflict) || bizerr.Is(err, CodeManagementNotAccessible) {
		return nil, bizerr.NewCode(CodePublicationIncomplete)
	}
	return nil, bizerr.WrapCode(err, CodePublicationUnavailable)
}

func (s *publicationServiceImpl) publishVersionInTransaction(
	ctx context.Context,
	versionNo string,
	actorUserID int,
) (*PublicationResult, error) {
	target, product, err := lockCatalogPublicationTarget(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if err = s.validatePublicationSource(ctx, target); err != nil {
		return nil, err
	}
	if err = validateCatalogPublicationCompleteness(ctx, product, target); err != nil {
		return nil, err
	}
	previous, err := lockCurrentPublishedVersion(ctx, product, target)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	supersededVersionNo := ""
	if previous != nil {
		supersededVersionNo = previous.VersionNo
		if err = supersedeCatalogVersion(ctx, previous.VersionNo); err != nil {
			return nil, err
		}
	}
	if err = publishCatalogTargetVersion(ctx, target, now); err != nil {
		return nil, err
	}
	if err = pointCatalogProductToVersion(ctx, product, target.VersionNo, actorUserID, now); err != nil {
		return nil, err
	}
	result := &PublicationResult{
		ProductNo: target.ProductNo, VersionNo: target.VersionNo, Status: "published",
		SupersededVersionNo: supersededVersionNo, PublishedAt: now,
	}
	if err = s.appendPublicationAudit(ctx, target, result); err != nil {
		return nil, err
	}
	return result, nil
}

func lockCatalogPublicationTarget(
	ctx context.Context,
	versionNo string,
) (*entity.CatalogProductVersion, *entity.CatalogProduct, error) {
	var target *entity.CatalogProductVersion
	if err := dao.CatalogProductVersion.Ctx(ctx).
		Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
		}).LockUpdate().Scan(&target); err != nil {
		return nil, nil, gerror.Wrap(err, "lock catalog product version for publication")
	}
	if target == nil {
		return nil, nil, bizerr.NewCode(CodePublicationNotAccessible)
	}
	if target.StatusCode != catalogStatusDraft || target.PublishedAt != nil || target.RetiredAt != nil {
		return nil, nil, bizerr.NewCode(CodePublicationState)
	}
	var product *entity.CatalogProduct
	if err := dao.CatalogProduct.Ctx(ctx).
		Where(do.CatalogProduct{
			TenantId: idccap.DefaultTenantID, ProductNo: target.ProductNo,
		}).
		LockUpdate().Scan(&product); err != nil {
		return nil, nil, gerror.Wrap(err, "lock catalog product for publication")
	}
	if product == nil {
		return nil, nil, bizerr.NewCode(CodePublicationNotAccessible)
	}
	if product.StatusCode != catalogStatusDraft && product.StatusCode != catalogStatusActive {
		return nil, nil, bizerr.NewCode(CodePublicationState)
	}
	if _, err := loadActiveCategoryForUpdate(ctx, product.CategoryNo); err != nil {
		if bizerr.Is(err, CodeManagementNotAccessible) {
			return nil, nil, bizerr.NewCode(CodePublicationIncomplete)
		}
		return nil, nil, err
	}
	return target, product, nil
}

func (s *publicationServiceImpl) validatePublicationSource(
	_ context.Context,
	target *entity.CatalogProductVersion,
) error {
	if target.SourceCode != catalogSourceManual && target.SourceCode != catalogSourceCopy &&
		target.SourceCode != "migration" {
		return bizerr.NewCode(CodePublicationState)
	}
	return nil
}

func lockCurrentPublishedVersion(
	ctx context.Context,
	product *entity.CatalogProduct,
	target *entity.CatalogProductVersion,
) (*entity.CatalogProductVersion, error) {
	versions := make([]entity.CatalogProductVersion, 0, 2)
	if err := dao.CatalogProductVersion.Ctx(ctx).
		Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, ProductNo: target.ProductNo, StatusCode: "published",
		}).WhereNot(dao.CatalogProductVersion.Columns().VersionNo, target.VersionNo).
		OrderAsc(dao.CatalogProductVersion.Columns().Id).
		Limit(2).LockUpdate().Scan(&versions); err != nil {
		return nil, gerror.Wrap(err, "lock current published catalog version")
	}
	if len(versions) > 1 {
		return nil, gerror.New("catalog product has multiple published versions")
	}
	if len(versions) == 0 {
		if product.CurrentVersionNo != nil {
			return nil, bizerr.NewCode(CodePublicationState)
		}
		return nil, nil
	}
	previous := &versions[0]
	if product.CurrentVersionNo == nil || *product.CurrentVersionNo != previous.VersionNo ||
		target.VersionNumber <= previous.VersionNumber {
		return nil, bizerr.NewCode(CodePublicationState)
	}
	return previous, nil
}

func supersedeCatalogVersion(ctx context.Context, versionNo string) error {
	affected, err := dao.CatalogProductVersion.Ctx(ctx).
		Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, VersionNo: versionNo, StatusCode: "published",
		}).Data(do.CatalogProductVersion{StatusCode: "superseded"}).UpdateAndGetAffected()
	if err != nil {
		return gerror.Wrap(err, "supersede current catalog version")
	}
	if affected != 1 {
		return bizerr.NewCode(CodePublicationState)
	}
	return nil
}

func publishCatalogTargetVersion(
	ctx context.Context,
	target *entity.CatalogProductVersion,
	publishedAt time.Time,
) error {
	affected, err := dao.CatalogProductVersion.Ctx(ctx).
		Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, VersionNo: target.VersionNo,
			StatusCode: catalogStatusDraft, SourceCode: target.SourceCode,
		}).Data(do.CatalogProductVersion{
		StatusCode: "published", PublishedAt: &publishedAt,
	}).UpdateAndGetAffected()
	if err != nil {
		return gerror.Wrap(err, "publish catalog product version")
	}
	if affected != 1 {
		return bizerr.NewCode(CodePublicationState)
	}
	return nil
}

func pointCatalogProductToVersion(
	ctx context.Context,
	product *entity.CatalogProduct,
	versionNo string,
	actorUserID int,
	_ time.Time,
) error {
	affected, err := dao.CatalogProduct.Ctx(ctx).
		Where(do.CatalogProduct{
			TenantId: idccap.DefaultTenantID, ProductNo: product.ProductNo, Version: product.Version,
		}).Data(do.CatalogProduct{
		StatusCode:       catalogStatusActive,
		CurrentVersionNo: &versionNo,
		UpdatedBy:        actorUserID,
		Version:          gdb.Raw(`"version" + 1`),
	}).UpdateAndGetAffected()
	if err != nil {
		return gerror.Wrap(err, "point product to published catalog version")
	}
	if affected != 1 {
		return bizerr.NewCode(CodePublicationState)
	}
	return nil
}

func (s *publicationServiceImpl) appendPublicationAudit(
	ctx context.Context,
	target *entity.CatalogProductVersion,
	result *PublicationResult,
) error {
	if err := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "catalog.product-version.publish", IdempotencyKey: target.VersionNo,
		OperationCode: "catalog.product-version.publish",
		TargetType:    "catalog_product_version", TargetNo: target.VersionNo,
		ResultCode: "succeeded", ReasonCode: target.SourceCode, RiskCode: "high",
		CorrelationNo:      target.SourceVersion,
		RequiredPermission: PermissionCatalogProductPublish,
		BeforeSnapshot: map[string]any{
			"state": target.StatusCode, "product_no": target.ProductNo,
			"superseded_version_no": result.SupersededVersionNo,
		},
		AfterSnapshot: map[string]any{
			"state": "published", "product_no": target.ProductNo,
			"version_no": target.VersionNo, "source": target.SourceCode,
		},
	}); err != nil {
		return gerror.Wrap(err, "append catalog product publication audit")
	}
	return nil
}
