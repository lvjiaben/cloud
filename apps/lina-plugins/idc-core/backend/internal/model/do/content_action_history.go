// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentActionHistory is the golang structure of table plugin_idc_core_content_action_history for DAO operations like Where/Data.
type ContentActionHistory struct {
	g.Meta         `orm:"table:plugin_idc_core_content_action_history, do:true"`
	Id             any        //
	TenantId       any        //
	ActionNo       any        //
	ContentNo      any        //
	ActionCode     any        //
	ActorUserId    any        //
	ContentVersion any        //
	OccurredAt     *time.Time //
}
