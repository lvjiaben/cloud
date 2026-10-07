// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentCategory is the golang structure of table plugin_idc_core_content_category for DAO operations like Where/Data.
type ContentCategory struct {
	g.Meta      `orm:"table:plugin_idc_core_content_category, do:true"`
	Id          any        //
	TenantId    any        //
	CategoryNo  any        //
	ContentType any        //
	Name        any        //
	SortOrder   any        //
	StatusCode  any        //
	Version     any        //
	CreatedBy   any        //
	UpdatedBy   any        //
	CreatedAt   *time.Time //
	UpdatedAt   *time.Time //
}
