// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EventPolicy is the golang structure of table plugin_idc_notify_event_policy for DAO operations like Where/Data.
type EventPolicy struct {
	g.Meta        `orm:"table:plugin_idc_notify_event_policy, do:true"`
	TenantId      any        //
	EventCode     any        //
	RecipientKind any        //
	ChannelCode   any        //
	Enabled       any        //
	Version       any        //
	UpdatedBy     any        //
	CreatedAt     *time.Time //
	UpdatedAt     *time.Time //
}
