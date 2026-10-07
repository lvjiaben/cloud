// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProduct is the golang structure of table plugin_idc_core_catalog_product for DAO operations like Where/Data.
type CatalogProduct struct {
	g.Meta           `orm:"table:plugin_idc_core_catalog_product, do:true"`
	Id               any        //
	TenantId         any        //
	ProductNo        any        //
	CategoryNo       any        //
	StatusCode       any        //
	PublicSlug       any        //
	NameZhCn         any        //
	NameEnUs         any        //
	SummaryZhCn      any        //
	SummaryEnUs      any        //
	CurrentVersionNo *string    //
	SortOrder        any        //
	Version          any        //
	CreatedBy        any        //
	UpdatedBy        any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
	DeletedAt        *time.Time //
}
