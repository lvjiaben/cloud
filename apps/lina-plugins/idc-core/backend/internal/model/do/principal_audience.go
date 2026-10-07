// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrincipalAudience is the golang structure of table plugin_idc_core_principal_audience for DAO operations like Where/Data.
type PrincipalAudience struct {
	g.Meta       `orm:"table:plugin_idc_core_principal_audience, do:true"`
	Id           any        //
	TenantId     any        //
	SysUserId    any        // Host sys_user identity ID validated through host capabilities
	AudienceCode any        // Audience code: customer or staff
	StatusCode   any        // Binding status code: active or revoked
	BoundAt      *time.Time // Time when the audience binding became active
	RevokedAt    *time.Time // Time when the audience binding was revoked
	Version      any        //
	CreatedBy    any        //
	UpdatedBy    any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
