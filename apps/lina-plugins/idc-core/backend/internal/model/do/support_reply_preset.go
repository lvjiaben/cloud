// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportReplyPreset is the golang structure of table plugin_idc_core_support_reply_preset for DAO operations like Where/Data.
type SupportReplyPreset struct {
	g.Meta       `orm:"table:plugin_idc_core_support_reply_preset, do:true"`
	Id           any        //
	TenantId     any        //
	PresetNo     any        //
	DepartmentNo any        //
	Locale       any        //
	Title        any        //
	Body         any        //
	SortOrder    any        //
	StatusCode   any        //
	Version      any        //
	CreatedBy    any        //
	UpdatedBy    any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
