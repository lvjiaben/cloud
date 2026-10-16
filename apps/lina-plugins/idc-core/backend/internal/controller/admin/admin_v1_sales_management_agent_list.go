// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementAgentList(ctx context.Context, req *v1.SalesManagementAgentListReq) (*v1.SalesManagementAgentListRes, error) {
	page, err := c.salesManagement.ListAgents(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementAgentListRes{Items: []v1.SalesManagementAgent{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementAgent(item))
	}
	return res, nil
}
