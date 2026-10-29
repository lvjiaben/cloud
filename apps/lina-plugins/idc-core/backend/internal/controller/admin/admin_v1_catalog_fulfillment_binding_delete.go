// This file handles idempotent removal of a draft product version's fulfillment binding.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogFulfillmentBindingDelete(ctx context.Context, req *v1.CatalogFulfillmentBindingDeleteReq) (res *v1.CatalogFulfillmentBindingDeleteRes, err error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogFulfillmentBindingDeleteRes, error) {
		if err := c.productAutomation.Unbind(ctx, req.VersionNo); err != nil {
			return nil, err
		}
		return &v1.CatalogFulfillmentBindingDeleteRes{}, nil
	})
}
