// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionCost is the golang structure of table plugin_idc_core_catalog_product_version_cost for DAO operations like Where/Data.
type CatalogProductVersionCost struct {
	g.Meta         `orm:"table:plugin_idc_core_catalog_product_version_cost, do:true"`
	Id             any        //
	TenantId       any        //
	CostNo         any        //
	VersionNo      any        //
	CurrencyCode   any        //
	PeriodUnitCode any        //
	PeriodCount    any        //
	UnitCostMinor  any        //
	EffectiveFrom  *time.Time //
	EffectiveUntil *time.Time //
	Version        any        //
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
}
