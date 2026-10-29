// This file delegates draft schedule deletion to the catalog permission and transaction boundary.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogPriceScheduleDelete(ctx context.Context, req *v1.CatalogPriceScheduleDeleteReq) (*v1.CatalogPriceScheduleDeleteRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogPriceScheduleDeleteRes, error) {
		if err := c.catalogWriter.DeletePriceSchedule(ctx, req.VersionNo, req.PriceNo); err != nil {
			return nil, err
		}
		return &v1.CatalogPriceScheduleDeleteRes{}, nil
	})
}
