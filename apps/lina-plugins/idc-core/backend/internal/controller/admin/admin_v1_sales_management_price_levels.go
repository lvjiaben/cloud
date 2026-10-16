// This generated controller delegates authorized price management to its owner.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SalesManagementPriceLevels(ctx context.Context, req *v1.SalesManagementPriceLevelsReq) (res *v1.SalesManagementPriceLevelsRes, err error) {
	page, err := c.salesManagement.PriceLevelCandidates(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res = &v1.SalesManagementPriceLevelsRes{Items: []v1.SalesManagementLevel{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, v1.SalesManagementLevel{PercentageBasisPoints: item.PercentageBasisPoints, PricingVersion: item.PricingVersion, LevelNo: item.LevelNo, Name: item.Name, EnglishName: item.EnglishName, Status: item.Status, Version: item.Version})
	}
	return res, nil
}
