// This file returns an owner-scoped page with its independent exact total.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

// TrafficPackageList maps bounded package facts without per-row owner queries.
func (c *ControllerV1) TrafficPackageList(ctx context.Context, req *v1.TrafficPackageListReq) (res *v1.TrafficPackageListRes, err error) {
	page, err := c.accessoryPricing.ListTrafficPackages(ctx, serveroperation.TrafficPackageSearch{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.TrafficPackage, len(page.Items))
	for i := range page.Items {
		items[i] = trafficPackageDTO(&page.Items[i])
	}
	return &v1.TrafficPackageListRes{Items: items, Total: page.Total}, nil
}
