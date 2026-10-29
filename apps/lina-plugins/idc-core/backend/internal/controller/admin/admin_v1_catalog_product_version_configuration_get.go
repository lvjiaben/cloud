package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogProductVersionConfigurationGet(ctx context.Context, req *v1.CatalogProductVersionConfigurationGetReq) (*v1.CatalogProductVersionConfigurationGetRes, error) {
	configuration, err := c.productAutomation.GetMerchantConfiguration(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductVersionConfigurationGetRes{Configuration: mapProductVersionConfiguration(configuration)}, nil
}
