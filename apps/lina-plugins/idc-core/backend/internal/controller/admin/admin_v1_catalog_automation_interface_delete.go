// This file handles audited deletion of an unreferenced automation interface.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceDelete(ctx context.Context, req *v1.CatalogAutomationInterfaceDeleteReq) (res *v1.CatalogAutomationInterfaceDeleteRes, err error) {
	if err := c.productAutomation.DeleteInterface(ctx, productautomation.DeleteInterfaceInput{InterfaceInstanceNo: req.InterfaceInstanceNo, ExpectedVersion: req.ExpectedVersion}); err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceDeleteRes{}, nil
}
