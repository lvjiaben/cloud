// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EventMaterialization is the golang structure of table plugin_idc_notify_event_materialization for DAO operations like Where/Data.
type EventMaterialization struct {
	g.Meta            `orm:"table:plugin_idc_notify_event_materialization, do:true"`
	TenantId          any        //
	MaterializationNo any        //
	SourcePlugin      any        //
	SourceEventNo     any        //
	EventCode         any        //
	RecipientKind     any        //
	RecipientDigest   any        //
	ChannelCode       any        //
	PolicyVersion     any        //
	StatusCode        any        //
	ReasonCode        any        //
	TaskNo            any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
