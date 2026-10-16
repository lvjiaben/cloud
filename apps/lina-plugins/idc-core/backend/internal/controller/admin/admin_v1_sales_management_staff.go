// This generated controller returns bounded sales management candidates.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementStaff(ctx context.Context, req *v1.SalesManagementStaffReq) (*v1.SalesManagementStaffRes, error) {
	page, err := c.salesManagement.StaffCandidates(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementStaffRes{Items: []v1.SalesManagementStaff{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, v1.SalesManagementStaff{UserID: item.UserID, Name: item.Name, Selectable: item.Selectable})
	}
	return res, nil
}
