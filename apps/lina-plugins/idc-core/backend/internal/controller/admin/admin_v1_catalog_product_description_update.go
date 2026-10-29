// This file delegates metadata updates to the transaction-owning catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/catalogdraft"
)

func (c *ControllerV1) CatalogProductDescriptionUpdate(ctx context.Context, req *v1.CatalogProductDescriptionUpdateReq) (res *v1.CatalogProductDescriptionUpdateRes, err error) {
	input := catalogdraft.BasicSettingsInput{Description: catalog.ProductDescriptionUpdateInput{SortOrder: req.SortOrder, CategoryNo: req.CategoryNo, DirectoryNo: req.DirectoryNo, ProductNo: req.ProductNo, Version: req.Version, NameZhCN: req.NameZhCn, NameEnUS: req.NameEnUs, SummaryZhCN: req.SummaryZhCn, SummaryEnUS: req.SummaryEnUs}, VersionNo: req.VersionNo, AddonProductNos: req.AddonProductNos}
	if req.Inventory != nil {
		input.Inventory = &catalog.ProductVersionInventoryUpdateInput{TrackingEnabled: req.Inventory.TrackingEnabled, SellableQuantity: req.Inventory.SellableQuantity, ExpectedVersion: req.Inventory.ExpectedVersion}
	}
	if req.LoginRules != nil {
		input.LoginRules = &catalogdraft.BasicLoginRulesInput{ExpectedVersion: req.LoginRules.ExpectedVersion, Values: mapConfigurationInput(req.LoginRules.Values)}
	}
	version, err := catalogdraft.SaveBasicSettings(ctx, input, c.catalogWriter, c.catalogPublication, c.productAutomation)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductDescriptionUpdateRes{Version: version}, nil
}
