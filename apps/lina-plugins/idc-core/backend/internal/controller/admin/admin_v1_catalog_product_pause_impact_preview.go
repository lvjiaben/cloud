package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

// CatalogProductPauseImpactPreview is advisory only: it reports the boundary
// of stopping new sales and never changes product or existing service state.
func (c *ControllerV1) CatalogProductPauseImpactPreview(
	ctx context.Context,
	req *v1.CatalogProductPauseImpactPreviewReq,
) (*v1.CatalogProductPauseImpactPreviewRes, error) {
	if req == nil {
		return nil, gerror.New("product pause preview request is required")
	}
	value, err := c.catalogManagement.PreviewProductPauseImpact(ctx, req.ProductNo)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductPauseImpactPreviewRes{Preview: &v1.CatalogProductPauseImpactPreview{
		ProductNo: value.ProductNo, Status: value.Status,
		NewSalesBlocked: value.NewSalesBlocked, ActiveCartItems: value.ActiveCartItems,
		UnpaidOrders: value.UnpaidOrders, PaidOrdersInProgress: value.PaidOrdersInProgress,
		ExistingServices: value.ExistingServices, ExistingServicesProtected: value.ExistingServicesProtected,
	}}, nil
}
