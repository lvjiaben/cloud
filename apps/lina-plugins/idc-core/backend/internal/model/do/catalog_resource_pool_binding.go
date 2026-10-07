// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogResourcePoolBinding is the golang structure of table plugin_idc_core_catalog_resource_pool_binding for DAO operations like Where/Data.
type CatalogResourcePoolBinding struct {
	g.Meta                  `orm:"table:plugin_idc_core_catalog_resource_pool_binding, do:true"`
	Id                      any        //
	TenantId                any        //
	BindingNo               any        //
	VersionNo               any        //
	PoolNo                  any        // Opaque resource-owner pool business number; no cross-plugin foreign key
	RegionCode              any        //
	LineCode                any        //
	Priority                any        //
	Weight                  any        //
	NewOrderEnabled         any        //
	StatusCode              any        //
	CreatedAt               *time.Time //
	MaintenanceModeCode     any        // Normal, drain, or maintenance mode for new-order routing only
	MinimumSellableQuantity any        // Local sellable quantity threshold required before automatic fulfillment is promised
	UncertainSalePolicyCode any        // Fail-closed stop-sale or explicit manual-confirmation policy for uncertain inventory
}
