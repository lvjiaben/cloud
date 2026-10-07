package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceConfigurationUpdate(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationUpdateReq) (*v1.CatalogAutomationInterfaceConfigurationUpdateRes, error) {
	configuration, err := c.productAutomation.UpdateInterfaceConfiguration(ctx, productautomation.UpdateInterfaceConfigurationInput{InterfaceInstanceNo: req.InterfaceInstanceNo, Values: mapConfigurationInput(req.Values), ExpectedRevision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceConfigurationUpdateRes{Configuration: mapInterfaceConfiguration(configuration)}, nil
}
