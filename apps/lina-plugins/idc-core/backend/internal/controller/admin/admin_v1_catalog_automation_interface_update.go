// This file handles future-facing automation-interface display-name changes.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceUpdate(ctx context.Context, req *v1.CatalogAutomationInterfaceUpdateReq) (res *v1.CatalogAutomationInterfaceUpdateRes, err error) {
	item, err := c.productAutomation.RenameInterface(ctx, productautomation.RenameInterfaceInput{InterfaceInstanceNo: req.InterfaceInstanceNo, DisplayName: req.DisplayName, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceUpdateRes{Interface: mapAutomationInterface(item)}, nil
}
