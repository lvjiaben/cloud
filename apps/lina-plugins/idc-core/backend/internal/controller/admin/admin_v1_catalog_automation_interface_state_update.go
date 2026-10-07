// This file handles permission-checked stopping and resuming of interface new business.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceStateUpdate(ctx context.Context, req *v1.CatalogAutomationInterfaceStateUpdateReq) (res *v1.CatalogAutomationInterfaceStateUpdateRes, err error) {
	item, err := c.productAutomation.UpdateNewBusinessState(ctx, productautomation.InterfaceStateInput{InterfaceInstanceNo: req.InterfaceInstanceNo, NewBusinessState: req.NewBusinessState, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceStateUpdateRes{Interface: mapAutomationInterface(item)}, nil
}
