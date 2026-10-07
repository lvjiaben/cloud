// This file projects bounded product labels for authorized package settings.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// TrafficPackageProductList delegates catalog eligibility to the business owner.
func (c *ControllerV1) TrafficPackageProductList(ctx context.Context, req *v1.TrafficPackageProductListReq) (res *v1.TrafficPackageProductListRes, err error) {
	products, err := c.accessoryPricing.TrafficPackageProducts(ctx, req.Keyword, req.ProductNos)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.TrafficPackageProductChoice, len(products))
	for i, product := range products {
		items[i] = &v1.TrafficPackageProductChoice{ProductNo: product.ProductNo, NameZhCN: product.NameZhCN, NameEnUS: product.NameEnUS}
	}
	return &v1.TrafficPackageProductListRes{Items: items}, nil
}
