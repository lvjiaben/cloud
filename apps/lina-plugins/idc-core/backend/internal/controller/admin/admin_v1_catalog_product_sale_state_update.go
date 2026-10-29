// This generated route changes sale state through the publication authority.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductSaleStateUpdate(ctx context.Context, req *v1.CatalogProductSaleStateUpdateReq) (*v1.CatalogProductSaleStateUpdateRes, error) {
	revision, err := c.catalogPublication.SetSaleState(ctx, catalog.ProductSaleStateInput{ProductNo: req.ProductNo, VersionNo: req.VersionNo, ExpectedVersion: req.ExpectedVersion, Active: req.Active})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductSaleStateUpdateRes{Version: revision}, nil
}
