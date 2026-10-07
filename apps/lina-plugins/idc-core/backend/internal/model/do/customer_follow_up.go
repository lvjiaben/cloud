// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerFollowUp is the golang structure of table plugin_idc_core_customer_follow_up for DAO operations like Where/Data.
type CustomerFollowUp struct {
	g.Meta         `orm:"table:plugin_idc_core_customer_follow_up, do:true"`
	Id             any        //
	TenantId       any        //
	FollowUpNo     any        //
	CustomerNo     any        //
	KindCode       any        //
	ChannelCode    any        //
	StatusCode     any        //
	Subject        any        //
	Detail         any        //
	OutcomeCode    any        //
	AssignedUserId any        //
	PlannedAt      *time.Time //
	CompletedAt    *time.Time //
	CancelledAt    *time.Time //
	CreatedBy      any        //
	UpdatedBy      any        //
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
}
