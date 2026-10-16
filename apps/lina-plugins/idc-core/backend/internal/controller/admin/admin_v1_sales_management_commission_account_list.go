// This generated controller reads scoped commission records.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementCommissionAccountList(ctx context.Context, req *v1.SalesManagementCommissionAccountListReq) (*v1.SalesManagementCommissionAccountListRes, error) {
	owner, ok := c.salesManagement.(salesmanagement.CommissionService)
	if !ok {
		return nil, gerror.New("commission service unavailable")
	}
	page, err := owner.ListCommissionAccounts(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementCommissionAccountListRes{Items: []v1.SalesManagementCommissionAccount{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementCommissionAccount(item))
	}
	return res, nil
}
