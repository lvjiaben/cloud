// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// WecomEmployeeBinding is the golang structure of table plugin_idc_notify_wecom_employee_binding for DAO operations like Where/Data.
type WecomEmployeeBinding struct {
	g.Meta                  `orm:"table:plugin_idc_notify_wecom_employee_binding, do:true"`
	TenantId                any        //
	BindingNo               any        //
	HostUserId              any        //
	StaffVersion            any        //
	CorpId                  any        //
	ChannelGeneration       any        //
	IdentityDigest          any        //
	IdentitySecretRef       any        //
	IdentitySecretUpdatedAt any        //
	SecretOperationKey      any        //
	StatusCode              any        //
	TerminalStatusCode      any        //
	RequestNo               any        //
	AvailableAt             any        //
	LeaseToken              any        //
	LeaseExpiresAt          any        //
	AttemptCount            any        //
	LastErrorCode           any        //
	Version                 any        //
	BoundAt                 any        //
	TerminalAt              any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
