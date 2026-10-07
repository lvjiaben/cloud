// This file handles admitted automation-interface creation without exposing frozen host evidence.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceCreate(ctx context.Context, req *v1.CatalogAutomationInterfaceCreateReq) (res *v1.CatalogAutomationInterfaceCreateRes, err error) {
	item, err := c.productAutomation.CreateInterface(ctx, productautomation.CreateInterfaceInput{
		DisplayName: req.DisplayName, SelectionToken: req.SelectionToken, Configuration: mapConfigurationInput(req.Configuration),
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceCreateRes{Interface: mapAutomationInterface(item)}, nil
}
