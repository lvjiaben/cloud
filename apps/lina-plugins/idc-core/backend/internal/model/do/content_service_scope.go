// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentServiceScope is the golang structure of table plugin_idc_core_content_service_scope for DAO operations like Where/Data.
type ContentServiceScope struct {
	g.Meta    `orm:"table:plugin_idc_core_content_service_scope, do:true"`
	Id        any        //
	TenantId  any        //
	ContentNo any        //
	ServiceNo any        //
	CreatedAt *time.Time //
}
