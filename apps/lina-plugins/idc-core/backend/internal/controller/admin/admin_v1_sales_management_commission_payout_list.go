// This generated controller reads scoped commission records.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementCommissionPayoutList(ctx context.Context, req *v1.SalesManagementCommissionPayoutListReq) (*v1.SalesManagementCommissionPayoutListRes, error) {
	owner, ok := c.salesManagement.(salesmanagement.CommissionService)
	if !ok {
		return nil, gerror.New("commission service unavailable")
	}
	page, err := owner.ListCommissionPayouts(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementCommissionPayoutListRes{Items: []v1.SalesManagementCommissionPayout{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementCommissionPayout(item))
	}
	return res, nil
}
