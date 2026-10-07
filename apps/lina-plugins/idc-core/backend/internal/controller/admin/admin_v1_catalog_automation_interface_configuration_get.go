package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogAutomationInterfaceConfigurationGet(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationGetReq) (*v1.CatalogAutomationInterfaceConfigurationGetRes, error) {
	configuration, err := c.productAutomation.GetInterfaceConfiguration(ctx, req.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceConfigurationGetRes{Configuration: mapInterfaceConfiguration(configuration)}, nil
}
