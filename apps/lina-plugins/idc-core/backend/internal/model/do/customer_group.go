// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerGroup is the golang structure of table plugin_idc_core_customer_group for DAO operations like Where/Data.
type CustomerGroup struct {
	g.Meta          `orm:"table:plugin_idc_core_customer_group, do:true"`
	Id              any        //
	TenantId        any        //
	GroupNo         any        //
	GroupCode       any        //
	NameZhCn        any        //
	NameEnUs        any        //
	DescriptionZhCn any        //
	DescriptionEnUs any        //
	StatusCode      any        //
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
