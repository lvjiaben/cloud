// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryAudit is the golang structure of table plugin_idc_notify_delivery_audit for DAO operations like Where/Data.
type DeliveryAudit struct {
	g.Meta         `orm:"table:plugin_idc_notify_delivery_audit, do:true"`
	TenantId       any        //
	AuditNo        any        //
	TaskNo         any        //
	ActionCode     any        //
	ActorType      any        //
	ActorUserId    any        //
	ReasonCode     any        //
	PreviousStatus any        //
	CurrentStatus  any        //
	TaskVersion    any        //
	CreatedAt      *time.Time //
}
