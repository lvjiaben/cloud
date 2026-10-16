// This file delegates the visible product offer set to catalog management.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogProductAddonsGet(ctx context.Context, req *v1.CatalogProductAddonsGetReq) (*v1.CatalogProductAddonsGetRes, error) {
	rows, err := c.catalogWriter.GetProductAddons(ctx, req.ProductNo)
	if err != nil {
		return nil, err
	}
	result := &v1.CatalogProductAddonsGetRes{Addons: make([]*v1.CatalogProductAddon, 0, len(rows))}
	for _, row := range rows {
		result.Addons = append(result.Addons, &v1.CatalogProductAddon{ProductNo: row.ProductNo, NameZhCN: row.NameZhCN, NameEnUS: row.NameEnUS})
	}
	return result, nil
}
