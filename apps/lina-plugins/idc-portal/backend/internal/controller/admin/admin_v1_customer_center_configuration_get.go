// This file applies the website configuration operation through its shared service.
package admin

import (
	"context"
	"lina-plugin-idc-portal/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerCenterConfigurationGet(ctx context.Context, req *v1.CustomerCenterConfigurationGetReq) (*v1.CustomerCenterConfigurationGetRes, error) {
	view, err := c.customers.Get(ctx)
	if err != nil {
		return nil, err
	}
	plugins := make([]v1.CustomerCenterCandidate, 0, len(view.Plugins))
	for _, item := range view.Plugins {
		plugins = append(plugins, v1.CustomerCenterCandidate{Pages: item.Pages, PluginID: item.PluginID, Name: item.Name, Schema: item.Schema, Renderer: item.Renderer})
	}
	return &v1.CustomerCenterConfigurationGetRes{SelectedPlugin: view.SelectedPlugin, Common: v1.CustomerCenterCommon(view.Common), Config: view.Config, Configurations: view.Configurations, Renderer: view.Renderer, Plugins: plugins}, nil
}
