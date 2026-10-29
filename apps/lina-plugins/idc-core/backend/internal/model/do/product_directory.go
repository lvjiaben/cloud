// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductDirectory is the golang structure of table plugin_idc_core_product_directory for DAO operations like Where/Data.
type ProductDirectory struct {
	g.Meta            `orm:"table:plugin_idc_core_product_directory, do:true"`
	Id                any        //
	TenantId          any        //
	DirectoryNo       any        //
	NameZhCn          any        //
	NameEnUs          any        //
	SortOrder         any        //
	StatusCode        any        //
	Version           any        //
	CreatedBy         any        //
	UpdatedBy         any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
	NavigationType    any        //
	TargetUrl         any        //
	Icon              any        //
	ParentDirectoryNo any        // Parent menu reference; empty means top level; owner validates a maximum of two levels
}
