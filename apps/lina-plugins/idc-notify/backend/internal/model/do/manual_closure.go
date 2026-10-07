// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ManualClosure is the golang structure of table plugin_idc_notify_manual_closure for DAO operations like Where/Data.
type ManualClosure struct {
	g.Meta              `orm:"table:plugin_idc_notify_manual_closure, do:true"`
	TenantId            any        //
	ClosureNo           any        //
	TaskNo              any        //
	ActorUserId         any        //
	ReasonText          any        //
	ExpectedTaskVersion any        //
	CreatedAt           *time.Time //
}
