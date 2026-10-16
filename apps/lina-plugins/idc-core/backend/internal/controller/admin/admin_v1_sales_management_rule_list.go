// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementRuleList(ctx context.Context, req *v1.SalesManagementRuleListReq) (*v1.SalesManagementRuleListRes, error) {
	page, err := c.salesManagement.ListRules(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementRuleListRes{Items: []v1.SalesManagementRule{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementRule(item))
	}
	return res, nil
}
