// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ChannelOperation is the golang structure of table plugin_idc_notify_channel_operation for DAO operations like Where/Data.
type ChannelOperation struct {
	g.Meta            `orm:"table:plugin_idc_notify_channel_operation, do:true"`
	TenantId          any        //
	OperationNo       any        //
	ChannelCode       any        //
	OperationCode     any        //
	ActorUserId       any        //
	ReasonCode        any        //
	BeforeVersion     any        //
	AfterVersion      any        //
	AffectedTaskCount any        //
	CreatedAt         *time.Time //
}
