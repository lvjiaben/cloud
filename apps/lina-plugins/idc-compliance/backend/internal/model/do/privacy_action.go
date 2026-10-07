// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyAction is the golang structure of table plugin_idc_compliance_privacy_action for DAO operations like Where/Data.
type PrivacyAction struct {
	g.Meta             `orm:"table:plugin_idc_compliance_privacy_action, do:true"`
	Id                 any        //
	TenantId           any        //
	ActionNo           any        //
	ActionCode         any        //
	TargetNo           any        //
	IdempotencyDigest  any        //
	RequestFingerprint any        //
	ResultReference    any        //
	ActorUserId        any        //
	CreatedAt          *time.Time //
}
