// This file maps simple product inventory updates to the catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// CatalogProductVersionInventoryUpdate updates a draft version's simple inventory setting.
func (c *ControllerV1) CatalogProductVersionInventoryUpdate(ctx context.Context, req *v1.CatalogProductVersionInventoryUpdateReq) (*v1.CatalogProductVersionInventoryUpdateRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogProductVersionInventoryUpdateRes, error) {
		result, err := c.catalogWriter.UpdateProductVersionInventory(ctx, catalog.ProductVersionInventoryUpdateInput{
			VersionNo: req.VersionNo, TrackingEnabled: req.TrackingEnabled,
			SellableQuantity: req.SellableQuantity, ExpectedVersion: req.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return &v1.CatalogProductVersionInventoryUpdateRes{Inventory: &v1.CatalogProductVersionInventory{
			TrackingEnabled: result.TrackingEnabled, SellableQuantity: result.SellableQuantity, Version: result.Version,
		}}, nil
	})
}
