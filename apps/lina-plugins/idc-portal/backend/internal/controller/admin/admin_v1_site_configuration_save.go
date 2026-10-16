// This file applies the website configuration operation through its shared service.
package admin

import (
	"context"
	"lina-plugin-idc-portal/backend/api/admin/v1"
	"lina-plugin-idc-portal/backend/internal/service/siteconfiguration"
)

func (c *ControllerV1) SiteConfigurationSave(ctx context.Context, req *v1.SiteConfigurationSaveReq) (*v1.SiteConfigurationSaveRes, error) {
	if err := c.sites.Save(ctx, siteconfiguration.Settings{Navigation: req.Navigation, SelectedPlugin: req.SelectedPlugin, Common: siteconfiguration.Common(req.Common), Config: req.Config}); err != nil {
		return nil, err
	}
	return &v1.SiteConfigurationSaveRes{Saved: true}, nil
}
