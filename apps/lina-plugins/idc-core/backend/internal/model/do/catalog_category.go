// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogCategory is the golang structure of table plugin_idc_core_catalog_category for DAO operations like Where/Data.
type CatalogCategory struct {
	g.Meta           `orm:"table:plugin_idc_core_catalog_category, do:true"`
	Id               any        //
	TenantId         any        //
	CategoryNo       any        //
	ParentCategoryNo *string    //
	NameZhCn         any        //
	NameEnUs         any        //
	SortOrder        any        //
	StatusCode       any        //
	Version          any        //
	CreatedBy        any        //
	UpdatedBy        any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
	DeletedAt        *time.Time //
}
