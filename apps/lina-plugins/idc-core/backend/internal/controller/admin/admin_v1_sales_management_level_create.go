// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementLevelCreate(ctx context.Context, req *v1.SalesManagementLevelCreateReq) (*v1.SalesManagementLevelCreateRes, error) {
	item, err := c.salesManagement.SaveLevel(ctx, salesmanagement.LevelInput{LevelNo: req.LevelNo, Name: req.Name, EnglishName: req.EnglishName, Status: req.Status, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementLevelCreateRes{Item: salesManagementLevel(item)}, nil
}
