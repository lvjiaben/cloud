// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportCustomStatus is the golang structure of table plugin_idc_core_support_custom_status for DAO operations like Where/Data.
type SupportCustomStatus struct {
	g.Meta     `orm:"table:plugin_idc_core_support_custom_status, do:true"`
	Id         any        //
	TenantId   any        //
	StatusNo   any        //
	NameZhCn   any        //
	NameEnUs   any        //
	BaseStage  any        //
	SortOrder  any        //
	StatusCode any        //
	Version    any        //
	CreatedBy  any        //
	UpdatedBy  any        //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
