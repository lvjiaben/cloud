// This file updates the simple inventory setting owned by a draft product version.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// ProductVersionInventory is the current simple inventory setting.
type ProductVersionInventory struct {
	TrackingEnabled  bool
	SellableQuantity int64
	Version          int64
}

// ProductVersionInventoryUpdateInput changes one draft version with optimistic concurrency.
type ProductVersionInventoryUpdateInput struct {
	VersionNo        string
	TrackingEnabled  bool
	SellableQuantity int64
	ExpectedVersion  int64
}

// UpdateProductVersionInventory changes only the product-owned inventory setting.
func (s *managementServiceImpl) UpdateProductVersionInventory(ctx context.Context, input ProductVersionInventoryUpdateInput) (*ProductVersionInventory, error) {
	actorUserID, err := s.authorizer.AuthorizeProductCreate(ctx)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if !existingCatalogReferencePattern.MatchString(input.VersionNo) || input.SellableQuantity < 0 || input.ExpectedVersion < 1 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	result, err := g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory inventory
SET tracking_enabled=?,sellable_quantity=?,version=version+1,updated_at=CURRENT_TIMESTAMP
FROM plugin_idc_core_catalog_product_version product_version
WHERE inventory.version_no=? AND inventory.version=?
  AND product_version.version_no=inventory.version_no AND product_version.tenant_id=?
  AND product_version.status_code='draft'`, input.TrackingEnabled, input.SellableQuantity,
		input.VersionNo, input.ExpectedVersion, idccap.DefaultTenantID)
	if err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "update product inventory"), CodeManagementUnavailable)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if rows != 1 {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	var inventory ProductVersionInventory
	if err = g.DB().GetScan(ctx, &inventory, `SELECT tracking_enabled,sellable_quantity,version
FROM plugin_idc_core_catalog_product_version_inventory WHERE version_no=?`, input.VersionNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return &inventory, nil
}
