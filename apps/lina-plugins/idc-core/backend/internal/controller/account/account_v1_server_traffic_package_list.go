// This file exposes customer-owned package choices without global sale facts.

package account

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

// ServerTrafficPackageList trusts the route and lets the owner resolve identity.
func (c *ControllerV1) ServerTrafficPackageList(ctx context.Context, req *v1.ServerTrafficPackageListReq) (res *v1.ServerTrafficPackageListRes, err error) {
	serviceNo := req.ServiceNo
	if request := g.RequestFromCtx(ctx); request != nil {
		serviceNo = request.GetRouter("serviceNo").String()
	}
	page, err := c.accessoryPricing.CustomerTrafficPackages(ctx, serviceNo, serveroperation.TrafficPackageSearch{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ServerTrafficPackage, len(page.Items))
	for i, item := range page.Items {
		items[i] = &v1.ServerTrafficPackage{PackageNo: item.PackageNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, TrafficGB: item.TrafficGB, CurrencyCode: item.CurrencyCode, AmountMinor: item.AmountMinor, Available: item.RemainingQuantity == nil || *item.RemainingQuantity > 0}
	}
	return &v1.ServerTrafficPackageListRes{Items: items, Total: page.Total}, nil
}
