// This file delegates product publication and maps its committed timestamp.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// CatalogProductPublish publishes one complete draft product version.
func (c *ControllerV1) CatalogProductPublish(
	ctx context.Context,
	req *v1.CatalogProductPublishReq,
) (res *v1.CatalogProductPublishRes, err error) {
	result, err := c.catalogPublication.PublishVersion(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductPublishRes{
		ProductNo: result.ProductNo, VersionNo: result.VersionNo, Status: result.Status,
		SupersededVersionNo: result.SupersededVersionNo,
		PublishedAt:         result.PublishedAt.UnixMilli(),
	}, nil
}
