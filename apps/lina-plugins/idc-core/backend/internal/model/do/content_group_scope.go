// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentGroupScope is the golang structure of table plugin_idc_core_content_group_scope for DAO operations like Where/Data.
type ContentGroupScope struct {
	g.Meta    `orm:"table:plugin_idc_core_content_group_scope, do:true"`
	Id        any        //
	TenantId  any        //
	ContentNo any        //
	GroupNo   any        //
	CreatedAt *time.Time //
}
