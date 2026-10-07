// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SecurityCodePurpose is the golang structure of table plugin_idc_notify_security_code_purpose for DAO operations like Where/Data.
type SecurityCodePurpose struct {
	g.Meta         `orm:"table:plugin_idc_notify_security_code_purpose, do:true"`
	TenantId       any        //
	CallerPluginId any        //
	PurposeCode    any        //
	EventCode      any        //
	Enabled        any        //
	Version        any        //
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
}
