// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentLevel is the golang structure of table plugin_idc_core_sales_agent_level for DAO operations like Where/Data.
type SalesAgentLevel struct {
	g.Meta     `orm:"table:plugin_idc_core_sales_agent_level, do:true"`
	Id         any        //
	TenantId   any        //
	LevelNo    any        //
	LevelCode  any        //
	NameZhCn   any        //
	NameEnUs   any        //
	SortOrder  any        //
	StatusCode any        //
	Version    any        //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
