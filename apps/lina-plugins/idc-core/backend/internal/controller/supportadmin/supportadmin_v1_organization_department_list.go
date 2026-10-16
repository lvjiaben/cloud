// This controller projects scoped organization choices without exposing organization storage.

package supportadmin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SupportOrganizationDepartmentList(ctx context.Context, req *v1.SupportOrganizationDepartmentListReq) (res *v1.SupportOrganizationDepartmentListRes, err error) {
	page, err := c.service.OrganizationDepartments(ctx, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]v1.SupportOrganizationDepartment, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.SupportOrganizationDepartment{DeptID: item.DeptID, DeptName: item.DeptName, DeptCode: item.DeptCode})
	}
	return &v1.SupportOrganizationDepartmentListRes{Items: items, Total: page.Total}, nil
}
