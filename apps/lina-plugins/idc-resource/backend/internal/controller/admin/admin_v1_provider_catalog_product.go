// This file returns only the selected supplier product's safe sales projection.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/api/admin/v1"
)

func (c *ControllerV1) ProviderCatalogProduct(ctx context.Context, req *v1.ProviderCatalogProductReq) (res *v1.ProviderCatalogProductRes, err error) {
	item, err := c.catalog.ReadCatalogProduct(ctx, req.InterfaceNo, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: req.Reference})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderCatalogProductRes{Item: item}, nil
}
