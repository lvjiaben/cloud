package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogProductVersionConfigurationUpdate(ctx context.Context, req *v1.CatalogProductVersionConfigurationUpdateReq) (*v1.CatalogProductVersionConfigurationUpdateRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogProductVersionConfigurationUpdateRes, error) {
		if req.MerchantValues == nil || len(req.PriceRows) > 256 {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		err := c.productAutomation.SaveVersionConfiguration(ctx, productautomation.SaveVersionConfigurationInput{VersionNo: req.VersionNo, MerchantValues: mapConfigurationInput(req.MerchantValues), ExpectedVersion: req.ExpectedVersion, CustomerFieldSelections: mapCustomerFieldSelections(req.CustomerFieldSelections)})
		if err != nil {
			return nil, err
		}
		if len(req.PriceRows) > 0 {
			rows, targets, mapErr := merchantConfigurationPriceRows(req.PriceRows)
			if mapErr != nil {
				return nil, mapErr
			}
			source := merchantConfigurationPriceSource{service: c.productAutomation, targets: targets}
			if priceErr := c.catalogWriter.SaveConfigurationPriceBatch(ctx, req.VersionNo, rows, source); priceErr != nil {
				return nil, priceErr
			}
		}
		configuration, err := c.productAutomation.GetMerchantConfiguration(ctx, req.VersionNo)
		if err != nil {
			return nil, err
		}
		return &v1.CatalogProductVersionConfigurationUpdateRes{Configuration: mapProductVersionConfiguration(configuration)}, nil
	})
}
