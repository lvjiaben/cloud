// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ManualResend is the golang structure of table plugin_idc_notify_manual_resend for DAO operations like Where/Data.
type ManualResend struct {
	g.Meta            `orm:"table:plugin_idc_notify_manual_resend, do:true"`
	TenantId          any        //
	OperationNo       any        //
	ChainNo           any        //
	SourceTaskNo      any        //
	CreatedTaskNo     any        //
	ActorUserId       any        //
	ReasonText        any        //
	SourceTaskVersion any        //
	CreatedAt         *time.Time //
}
