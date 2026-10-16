// This file projects only safe public presentation settings of the active customer template.
package public

import (
	"context"
	adminv1 "lina-plugin-idc-portal/backend/api/admin/v1"
	"lina-plugin-idc-portal/backend/api/public/v1"
)

// CustomerCenterConfigurationGet resolves current active template presentation.
func (c *ControllerV1) CustomerCenterConfigurationGet(ctx context.Context, _ *v1.CustomerCenterConfigurationGetReq) (*v1.CustomerCenterConfigurationGetRes, error) {
	view, active, err := c.customers.Public(ctx)
	if err != nil {
		return nil, err
	}
	entryURL := ""
	if view.Entry != nil {
		entryURL = view.Entry.URL
	}
	return &v1.CustomerCenterConfigurationGetRes{Entry: view.Entry, EntryURL: entryURL, Active: active, SelectedPlugin: view.SelectedPlugin, Common: adminv1.CustomerCenterCommon(view.Common), Config: view.Config, Renderer: view.Renderer}, nil
}
