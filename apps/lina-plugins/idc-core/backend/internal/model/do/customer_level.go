// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLevel is the golang structure of table plugin_idc_core_customer_level for DAO operations like Where/Data.
type CustomerLevel struct {
	g.Meta          `orm:"table:plugin_idc_core_customer_level, do:true"`
	Id              any        //
	TenantId        any        //
	LevelNo         any        //
	LevelCode       any        //
	NameZhCn        any        //
	NameEnUs        any        //
	DescriptionZhCn any        //
	DescriptionEnUs any        //
	Priority        any        //
	IsDefault       any        //
	StatusCode      any        //
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
