// This generated route starts an editable revision through both catalog and automation owners.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductRevisionCreate(ctx context.Context, req *v1.CatalogProductRevisionCreateReq) (*v1.CatalogProductRevisionCreateRes, error) {
	result, err := c.catalogCopy.Copy(ctx, catalog.ProductCopyInput{Revision: true, SourceProductNo: req.ProductNo, SourceVersionNo: req.SourceVersionNo})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductRevisionCreateRes{CatalogProductCreateRes: v1.CatalogProductCreateRes{ProductNo: result.ProductNo, VersionNo: result.VersionNo, VersionNumber: result.VersionNumber, CategoryNo: result.CategoryNo, Status: result.Status, VersionStatus: result.VersionStatus, Source: result.Source}}, nil
}
