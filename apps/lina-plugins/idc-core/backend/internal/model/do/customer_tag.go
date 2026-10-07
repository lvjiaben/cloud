// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerTag is the golang structure of table plugin_idc_core_customer_tag for DAO operations like Where/Data.
type CustomerTag struct {
	g.Meta     `orm:"table:plugin_idc_core_customer_tag, do:true"`
	Id         any        //
	TenantId   any        //
	TagNo      any        //
	TagCode    any        //
	NameZhCn   any        //
	NameEnUs   any        //
	ColorToken any        //
	StatusCode any        //
	Version    any        //
	CreatedBy  any        //
	UpdatedBy  any        //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
