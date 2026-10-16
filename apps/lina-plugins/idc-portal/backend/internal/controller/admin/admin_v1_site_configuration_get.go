// This file applies the website configuration operation through its shared service.
package admin

import (
	"context"
	"lina-plugin-idc-portal/backend/api/admin/v1"
)

func (c *ControllerV1) SiteConfigurationGet(ctx context.Context, req *v1.SiteConfigurationGetReq) (*v1.SiteConfigurationGetRes, error) {
	view, err := c.sites.Get(ctx)
	if err != nil {
		return nil, err
	}
	plugins := make([]v1.WebsiteCandidate, 0, len(view.Plugins))
	for _, item := range view.Plugins {
		plugins = append(plugins, v1.WebsiteCandidate{Pages: item.Pages, PluginID: item.PluginID, Name: item.Name, Schema: item.Schema, Renderer: item.Renderer})
	}
	return &v1.SiteConfigurationGetRes{Navigation: view.Navigation, SelectedPlugin: view.SelectedPlugin, Common: v1.WebsiteCommon(view.Common), Config: view.Config, Configurations: view.Configurations, Renderer: view.Renderer, Plugins: plugins}, nil
}
