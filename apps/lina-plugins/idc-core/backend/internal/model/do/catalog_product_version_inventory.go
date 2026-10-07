// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionInventory is the golang structure of table plugin_idc_core_catalog_product_version_inventory for DAO operations like Where/Data.
type CatalogProductVersionInventory struct {
	g.Meta           `orm:"table:plugin_idc_core_catalog_product_version_inventory, do:true"`
	VersionNo        any        //
	TrackingEnabled  any        //
	SellableQuantity any        //
	Version          any        //
	UpdatedAt        *time.Time //
}
