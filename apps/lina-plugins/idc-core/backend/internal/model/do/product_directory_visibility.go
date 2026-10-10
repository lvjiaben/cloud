// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductDirectoryVisibility is the golang structure of table plugin_idc_core_product_directory_visibility for DAO operations like Where/Data.
type ProductDirectoryVisibility struct {
	g.Meta      `orm:"table:plugin_idc_core_product_directory_visibility, do:true"`
	TenantId    any        //
	CustomerNo  any        //
	DirectoryNo any        //
	Hidden      any        //
	UpdatedAt   *time.Time //
}
