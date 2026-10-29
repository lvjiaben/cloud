// This generated endpoint delegates recoverable product deletion to catalog.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductDelete(ctx context.Context, req *v1.CatalogProductDeleteReq) (res *v1.CatalogProductDeleteRes, err error) {
	if err = c.catalogWriter.DeleteProduct(ctx, catalog.ProductDeleteInput{ProductNo: req.ProductNo, Version: req.Version}); err != nil {
		return nil, err
	}
	return &v1.CatalogProductDeleteRes{}, nil
}
