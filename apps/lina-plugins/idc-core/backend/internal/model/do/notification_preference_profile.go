// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NotificationPreferenceProfile is the golang structure of table plugin_idc_core_notification_preference_profile for DAO operations like Where/Data.
type NotificationPreferenceProfile struct {
	g.Meta     `orm:"table:plugin_idc_core_notification_preference_profile, do:true"`
	Id         any        //
	TenantId   any        //
	CustomerNo any        //
	Version    any        //
	UpdatedAt  *time.Time //
}
