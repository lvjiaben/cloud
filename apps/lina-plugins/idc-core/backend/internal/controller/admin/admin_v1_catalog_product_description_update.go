// This file delegates metadata updates to the transaction-owning catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductDescriptionUpdate(ctx context.Context, req *v1.CatalogProductDescriptionUpdateReq) (res *v1.CatalogProductDescriptionUpdateRes, err error) {
	version, err := c.catalogWriter.UpdateProductDescription(ctx, catalog.ProductDescriptionUpdateInput{DirectoryNo: req.DirectoryNo, ProductNo: req.ProductNo, Version: req.Version, NameZhCN: req.NameZhCn, NameEnUS: req.NameEnUs, SummaryZhCN: req.SummaryZhCn, SummaryEnUS: req.SummaryEnUs})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductDescriptionUpdateRes{Version: version}, nil
}
