// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesGroup is the golang structure of table plugin_idc_core_sales_group for DAO operations like Where/Data.
type SalesGroup struct {
	g.Meta     `orm:"table:plugin_idc_core_sales_group, do:true"`
	Id         any        //
	TenantId   any        //
	GroupNo    any        //
	GroupCode  any        //
	NameZhCn   any        //
	NameEnUs   any        //
	StatusCode any        //
	Version    any        //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
