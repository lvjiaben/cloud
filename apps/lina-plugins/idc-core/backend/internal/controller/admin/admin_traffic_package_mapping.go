// This file maps traffic-package API values without leaking service entities.

package admin

import (
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

// trafficPackageInput shares only the create/update value conversion.
func trafficPackageInput(value v1.TrafficPackageFields) serveroperation.TrafficPackageInput {
	return serveroperation.TrafficPackageInput{NameZhCN: value.NameZhCN, NameEnUS: value.NameEnUS, CurrencyCode: value.CurrencyCode, TrafficGB: value.TrafficGB, AmountMinor: value.AmountMinor, Enabled: value.Enabled, StockQuantity: value.StockQuantity, ProductNos: value.ProductNos}
}

// trafficPackageDTO converts timestamps once at the HTTP boundary.
func trafficPackageDTO(value *serveroperation.TrafficPackage) *v1.TrafficPackage {
	if value == nil {
		return nil
	}
	return &v1.TrafficPackage{PackageNo: value.PackageNo, Version: value.Version, NameZhCN: value.NameZhCN, NameEnUS: value.NameEnUS, CurrencyCode: value.CurrencyCode, TrafficGB: value.TrafficGB, AmountMinor: value.AmountMinor, Enabled: value.Enabled, StockQuantity: value.StockQuantity, OccupiedQuantity: value.OccupiedQuantity, RemainingQuantity: value.RemainingQuantity, ProductNos: value.ProductNos, PublishedAt: value.PublishedAt.UnixMilli()}
}
