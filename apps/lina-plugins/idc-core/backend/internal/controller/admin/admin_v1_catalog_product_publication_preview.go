// This file maps bounded publication readiness to the staff API.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// CatalogProductPublicationPreview returns advisory publication readiness.
func (c *ControllerV1) CatalogProductPublicationPreview(
	ctx context.Context,
	req *v1.CatalogProductPublicationPreviewReq,
) (res *v1.CatalogProductPublicationPreviewRes, err error) {
	preview, err := c.catalogPublicationPreview.PreviewVersion(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	checks := make([]*v1.CatalogProductPublicationCheck, 0, len(preview.Checks))
	for _, check := range preview.Checks {
		checks = append(checks, &v1.CatalogProductPublicationCheck{
			Code: string(check.Code), Passed: check.Passed, ReasonCode: string(check.ReasonCode),
		})
	}
	return &v1.CatalogProductPublicationPreviewRes{
		ProductNo: preview.ProductNo, VersionNo: preview.VersionNo,
		Ready: preview.Ready, Checks: checks,
	}, nil
}
