// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementAgentCreate(ctx context.Context, req *v1.SalesManagementAgentCreateReq) (*v1.SalesManagementAgentCreateRes, error) {
	item, err := c.salesManagement.SaveAgent(ctx, salesmanagement.AgentInput{CustomerNo: req.CustomerNo, LevelNo: req.LevelNo, Status: req.Status, ExpiresAt: salesManagementTime(req.ExpiresAt), ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementAgentCreateRes{Item: salesManagementAgent(item)}, nil
}
