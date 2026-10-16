// This file maps the principal-scoped add-on directory to public DTOs.
package account

import (
	"context"
	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommerceAddonTargets(ctx context.Context, _ *v1.CommerceAddonTargetsReq) (*v1.CommerceAddonTargetsRes, error) {
	rows, err := c.commerce.ListAddonTargets(ctx)
	if err != nil {
		return nil, err
	}
	result := &v1.CommerceAddonTargetsRes{Targets: make([]*v1.CommerceAddonTarget, 0, len(rows))}
	for _, row := range rows {
		target := &v1.CommerceAddonTarget{EntitlementNo: row.EntitlementNo, ProductNameZhCN: row.ProductNameZhCN, ProductNameEnUS: row.ProductNameEnUS, Addons: make([]*v1.CommerceAddonOffer, 0, len(row.Addons))}
		for _, offer := range row.Addons {
			target.Addons = append(target.Addons, &v1.CommerceAddonOffer{PublicSlug: offer.PublicSlug, NameZhCN: offer.NameZhCN, NameEnUS: offer.NameEnUS})
		}
		result.Targets = append(result.Targets, target)
	}
	return result, nil
}
