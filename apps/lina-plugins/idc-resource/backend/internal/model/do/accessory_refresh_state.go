// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AccessoryRefreshState is the golang structure of table plugin_idc_resource_accessory_refresh_state for DAO operations like Where/Data.
type AccessoryRefreshState struct {
	g.Meta           `orm:"table:plugin_idc_resource_accessory_refresh_state, do:true"`
	Id               any        //
	TenantId         any        //
	MappingNo        any        //
	NextAttemptAt    *time.Time //
	AttemptCount     any        //
	LeaseTokenDigest any        //
	LeaseExpiresAt   *time.Time //
	FailureCode      any        //
	UpdatedAt        *time.Time //
}
