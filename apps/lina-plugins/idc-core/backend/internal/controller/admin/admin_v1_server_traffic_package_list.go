// This file projects governed staff traffic-package choices without exposing
// installation-wide inventory or accepting a customer identity from the caller.

package admin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

// ServerTrafficPackageList resolves the path target before the staff boundary.
func (c *ControllerV1) ServerTrafficPackageList(ctx context.Context, req *v1.ServerTrafficPackageListReq) (res *v1.ServerTrafficPackageListRes, err error) {
	serviceNo := req.ServiceNo
	if request := g.RequestFromCtx(ctx); request != nil {
		serviceNo = request.GetRouter("serviceNo").String()
	}
	page, err := c.serverAccessoryManagement.TrafficPackages(ctx, serviceNo, serveroperation.TrafficPackageSearch{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ServerTrafficPackage, len(page.Items))
	for i, item := range page.Items {
		items[i] = &v1.ServerTrafficPackage{PackageNo: item.PackageNo, Version: item.Version, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, TrafficGB: item.TrafficGB, CurrencyCode: item.CurrencyCode, AmountMinor: item.AmountMinor, Available: item.RemainingQuantity == nil || *item.RemainingQuantity > 0}
	}
	return &v1.ServerTrafficPackageListRes{Items: items, Total: page.Total}, nil
}
