// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogCostSnapshot is the golang structure of table plugin_idc_core_catalog_cost_snapshot for DAO operations like Where/Data.
type CatalogCostSnapshot struct {
	g.Meta           `orm:"table:plugin_idc_core_catalog_cost_snapshot, do:true"`
	Id               any        //
	TenantId         any        //
	CostSnapshotNo   any        //
	BindingNo        any        //
	SourceCode       any        //
	SourceSnapshotNo any        //
	CurrencyCode     any        //
	AmountMinor      any        //
	PeriodUnitCode   any        //
	PeriodCount      any        //
	ObservedAt       *time.Time //
	ValidUntil       *time.Time //
	CreatedBy        any        //
	CreatedAt        *time.Time //
}
