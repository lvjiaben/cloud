// This file applies the website configuration operation through its shared service.
package admin

import (
	"context"
	"lina-plugin-idc-portal/backend/api/admin/v1"
	"lina-plugin-idc-portal/backend/internal/service/customerconfiguration"
)

func (c *ControllerV1) CustomerCenterConfigurationSave(ctx context.Context, req *v1.CustomerCenterConfigurationSaveReq) (*v1.CustomerCenterConfigurationSaveRes, error) {
	if err := c.customers.Save(ctx, customerconfiguration.Settings{SelectedPlugin: req.SelectedPlugin, Common: customerconfiguration.Common(req.Common), Config: req.Config}); err != nil {
		return nil, err
	}
	return &v1.CustomerCenterConfigurationSaveRes{Saved: true}, nil
}
