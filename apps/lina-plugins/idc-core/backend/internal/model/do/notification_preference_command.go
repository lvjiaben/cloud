// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NotificationPreferenceCommand is the golang structure of table plugin_idc_core_notification_preference_command for DAO operations like Where/Data.
type NotificationPreferenceCommand struct {
	g.Meta             `orm:"table:plugin_idc_core_notification_preference_command, do:true"`
	Id                 any        //
	TenantId           any        //
	CommandNo          any        //
	CustomerNo         any        //
	IdempotencyDigest  any        //
	RequestFingerprint any        //
	EvidenceDigest     any        //
	ResultVersion      any        //
	AppliedAt          *time.Time //
}
