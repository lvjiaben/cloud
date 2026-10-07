// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// WecomBindingAudit is the golang structure of table plugin_idc_notify_wecom_binding_audit for DAO operations like Where/Data.
type WecomBindingAudit struct {
	g.Meta         `orm:"table:plugin_idc_notify_wecom_binding_audit, do:true"`
	TenantId       any        //
	AuditNo        any        //
	BindingNo      any        //
	ActorUserId    any        //
	ActionCode     any        //
	ReasonCode     any        //
	PreviousStatus any        //
	CurrentStatus  any        //
	BindingVersion any        //
	CreatedAt      *time.Time //
}
