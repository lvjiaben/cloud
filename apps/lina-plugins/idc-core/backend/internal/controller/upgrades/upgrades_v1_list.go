// This file maps customer-owned upgrade workflow results to stable HTTP projections.
package upgrades

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/upgrades/v1"
	"lina-plugin-idc-core/backend/internal/service/entitlementupgrade"
	"strconv"
)

func project(r *entitlementupgrade.View) *v1.Upgrade {
	return &v1.Upgrade{UpgradeNo: r.UpgradeNo, EntitlementNo: r.EntitlementNo, CurrencyCode: r.CurrencyCode, StatusCode: r.StatusCode, InvoiceNo: r.InvoiceNo, SourcePeriodMinor: strconv.FormatInt(r.SourcePeriodMinor, 10), TargetPeriodMinor: strconv.FormatInt(r.TargetPeriodMinor, 10), NetMinor: strconv.FormatInt(r.NetMinor, 10), TaxMinor: strconv.FormatInt(r.TaxMinor, 10), TotalMinor: strconv.FormatInt(r.TotalMinor, 10), ExpiresAt: r.ExpiresAt}
}

// List exposes only the current customer's bounded safe projection.
func (c *ControllerV1) List(ctx context.Context, _ *v1.ListReq) (*v1.ListRes, error) {
	p, err := c.service.List(ctx)
	if err != nil {
		return nil, err
	}
	out := &v1.ListRes{Targets: []*v1.Target{}, Items: []*v1.Upgrade{}}
	for _, t := range p.Targets {
		out.Targets = append(out.Targets, &v1.Target{EntitlementNo: t.EntitlementNo, PublicSlug: t.PublicSlug, NameZhCN: t.NameZhCN, NameEnUS: t.NameEnUS, CurrencyCode: t.CurrencyCode})
	}
	for i := range p.Items {
		out.Items = append(out.Items, project(&p.Items[i]))
	}
	return out, nil
}
