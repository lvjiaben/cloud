// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementOwnershipList(ctx context.Context, req *v1.SalesManagementOwnershipListReq) (*v1.SalesManagementOwnershipListRes, error) {
	page, err := c.salesManagement.ListOwnership(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementOwnershipListRes{Items: []v1.SalesManagementOwnership{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementOwnership(item))
	}
	return res, nil
}
