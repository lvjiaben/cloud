// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementOwnershipCreate(ctx context.Context, req *v1.SalesManagementOwnershipCreateReq) (*v1.SalesManagementOwnershipCreateRes, error) {
	item, err := c.salesManagement.SaveOwnership(ctx, salesmanagement.OwnershipInput{CustomerNo: req.CustomerNo, ReferrerCustomerNo: req.ReferrerCustomerNo, StaffUserID: req.StaffUserID, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementOwnershipCreateRes{Item: salesManagementOwnership(item)}, nil
}
