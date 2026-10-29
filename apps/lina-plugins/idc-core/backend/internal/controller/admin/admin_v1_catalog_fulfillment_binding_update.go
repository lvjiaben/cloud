// This file handles admitted creation or replacement of a draft product version's fulfillment binding.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogFulfillmentBindingUpdate(ctx context.Context, req *v1.CatalogFulfillmentBindingUpdateReq) (res *v1.CatalogFulfillmentBindingUpdateRes, err error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogFulfillmentBindingUpdateRes, error) {
		binding, err := c.productAutomation.Bind(ctx, productautomation.BindInput{VersionNo: req.VersionNo, InterfaceInstanceNo: req.InterfaceInstanceNo})
		if err != nil {
			return nil, err
		}
		return &v1.CatalogFulfillmentBindingUpdateRes{Binding: mapAutomationBinding(binding)}, nil
	})
}
