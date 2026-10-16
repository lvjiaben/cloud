// This generated controller reads scoped commission records.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementCommissionRecordList(ctx context.Context, req *v1.SalesManagementCommissionRecordListReq) (*v1.SalesManagementCommissionRecordListRes, error) {
	owner, ok := c.salesManagement.(salesmanagement.CommissionService)
	if !ok {
		return nil, gerror.New("commission service unavailable")
	}
	page, err := owner.ListCommissionRecords(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementCommissionRecordListRes{Items: []v1.SalesManagementCommissionRecord{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementCommissionRecord(item))
	}
	return res, nil
}
