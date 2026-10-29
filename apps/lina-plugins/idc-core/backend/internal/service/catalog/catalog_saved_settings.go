// This file applies local product settings immediately. Internal immutable
// versions preserve old orders; merchants do not manage publication records.

package catalog

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"time"
)

func (s *publicationServiceImpl) ApplySavedVersion(ctx context.Context, versionNo string) (*PublicationResult, error) {
	actor, err := s.authorizer.AuthorizeProductSave(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if actor <= 0 || !existingCatalogReferencePattern.MatchString(versionNo) {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	var result *PublicationResult
	err = dao.CatalogProduct.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, product, loadErr := lockCatalogVersionTarget(ctx, versionNo, false)
		if loadErr != nil {
			return loadErr
		}
		previous, loadErr := lockCurrentPublishedVersion(ctx, product, target)
		if loadErr != nil {
			return loadErr
		}
		now := time.Now()
		result = &PublicationResult{ProductNo: product.ProductNo, VersionNo: versionNo, Status: productSaleVersionPublished, PublishedAt: now}
		if previous != nil {
			result.SupersededVersionNo = previous.VersionNo
			// An untouched working copy follows the current stock budget; reservations
			// are counted across this product, so edits never mint stock.
			var previousInventory struct {
				TrackingEnabled  bool
				SellableQuantity int64
			}
			if loadErr = dao.CatalogProductVersionInventory.Ctx(ctx).Fields("tracking_enabled,sellable_quantity").Where(do.CatalogProductVersionInventory{VersionNo: previous.VersionNo}).LockUpdate().Scan(&previousInventory); loadErr != nil {
				return loadErr
			}
			if _, loadErr = dao.CatalogProductVersionInventory.Ctx(ctx).Where(do.CatalogProductVersionInventory{VersionNo: versionNo, Version: 1}).Data(do.CatalogProductVersionInventory{TrackingEnabled: previousInventory.TrackingEnabled, SellableQuantity: previousInventory.SellableQuantity}).Update(); loadErr != nil {
				return loadErr
			}

			if loadErr = supersedeCatalogVersion(ctx, previous.VersionNo); loadErr != nil {
				return loadErr
			}
		}
		if loadErr = publishCatalogTargetVersion(ctx, target, now); loadErr != nil {
			return loadErr
		}
		// Saving never changes the merchant's explicit sale switch. New products
		// start off sale until their real configuration and prices are ready.
		state := product.StatusCode
		if state == catalogStatusDraft {
			state = productSaleSuspended
		}
		updated, updateErr := dao.CatalogProduct.Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: product.ProductNo, Version: product.Version}).Data(do.CatalogProduct{CurrentVersionNo: &versionNo, StatusCode: state, Version: product.Version + 1, UpdatedBy: actor}).Update()
		if updateErr = requireSingleCatalogWrite(updated, updateErr, "apply saved product settings"); updateErr != nil {
			return updateErr
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.product.settings.save", IdempotencyKey: target.VersionNo,
			OperationCode: "catalog.product.settings.save", TargetType: "catalog_product", TargetNo: product.ProductNo,
			ResultCode: "succeeded", ReasonCode: "merchant_save", RiskCode: "medium", RequiredPermission: permissionCatalogProductUpdate,
			BeforeSnapshot: map[string]any{"current_version_no": result.SupersededVersionNo},
			AfterSnapshot:  map[string]any{"current_version_no": versionNo, "state": state},
		})
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return result, nil
}

func (s *publicationServiceImpl) WithSavedSettings(ctx context.Context, versionNo string, save func(context.Context) error) error {
	if save == nil {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	if actor, err := s.authorizer.AuthorizeProductSave(ctx, versionNo); err != nil {
		return err
	} else if actor <= 0 {
		return bizerr.NewCode(CodeManagementNotAccessible)
	}
	return dao.CatalogProduct.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := save(ctx); err != nil {
			return err
		}
		_, err := s.ApplySavedVersion(ctx, versionNo)
		return err
	})
}
