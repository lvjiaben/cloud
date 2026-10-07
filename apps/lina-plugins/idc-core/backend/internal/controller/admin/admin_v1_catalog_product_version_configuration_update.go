package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogProductVersionConfigurationUpdate(ctx context.Context, req *v1.CatalogProductVersionConfigurationUpdateReq) (*v1.CatalogProductVersionConfigurationUpdateRes, error) {
	err := c.productAutomation.SaveVersionConfiguration(ctx, productautomation.SaveVersionConfigurationInput{VersionNo: req.VersionNo, MerchantValues: mapConfigurationInput(req.MerchantValues), ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	configuration, err := c.productAutomation.GetVersionConfiguration(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductVersionConfigurationUpdateRes{Configuration: mapProductVersionConfiguration(configuration)}, nil
}
