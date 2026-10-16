// This file delegates product copying to startup-owned transactional owners.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/internal/service/catalog"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogProductCopy(ctx context.Context, req *v1.CatalogProductCopyReq) (res *v1.CatalogProductCopyRes, err error) {
	created, err := c.catalogCopy.Copy(ctx, catalog.ProductCopyInput{SourceProductNo: req.ProductNo, SourceVersionNo: req.SourceVersionNo, PublicSlug: req.PublicSlug, NameZhCN: req.NameZhCn, NameEnUS: req.NameEnUs})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductCopyRes{CatalogProductCreateRes: v1.CatalogProductCreateRes{ProductNo: created.ProductNo, VersionNo: created.VersionNo, VersionNumber: created.VersionNumber, CategoryNo: created.CategoryNo, Status: created.Status, VersionStatus: created.VersionStatus, Source: created.Source}}, nil
}
