// This file delegates manual product-draft creation to the transactional service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductCreate(
	ctx context.Context,
	req *v1.CatalogProductCreateReq,
) (*v1.CatalogProductCreateRes, error) {
	created, err := c.catalogWriter.CreateProductDraft(ctx, catalog.ProductCreateInput{
		DirectoryNo: req.DirectoryNo, CategoryNo: req.CategoryNo, PublicSlug: req.PublicSlug,
		NameZhCN: req.NameZhCn, NameEnUS: req.NameEnUs,
		SummaryZhCN: req.SummaryZhCn, SummaryEnUS: req.SummaryEnUs,
		SortOrder: req.SortOrder,

		InventoryTrackingEnabled: req.InventoryTrackingEnabled,
		SellableQuantity:         req.SellableQuantity,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductCreateRes{
		ProductNo: created.ProductNo, VersionNo: created.VersionNo,
		VersionNumber: created.VersionNumber, CategoryNo: created.CategoryNo,
		Status: created.Status, VersionStatus: created.VersionStatus, Source: created.Source,
	}, nil
}
