// This file delegates the audited offer-set replacement to its catalog owner.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductAddonsSet(ctx context.Context, req *v1.CatalogProductAddonsSetReq) (*v1.CatalogProductAddonsSetRes, error) {
	version, err := c.catalogWriter.SetProductAddons(ctx, catalog.ProductAddonsInput{ProductNo: req.ProductNo, ProductNos: req.ProductNos, Version: req.Version})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductAddonsSetRes{Version: version}, nil
}
