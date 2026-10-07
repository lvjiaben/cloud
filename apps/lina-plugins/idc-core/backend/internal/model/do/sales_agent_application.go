// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentApplication is the golang structure of table plugin_idc_core_sales_agent_application for DAO operations like Where/Data.
type SalesAgentApplication struct {
	g.Meta            `orm:"table:plugin_idc_core_sales_agent_application, do:true"`
	Id                any        //
	TenantId          any        //
	ApplicationNo     any        //
	CustomerNo        any        //
	RequestedLevelNo  any        //
	StatusCode        any        //
	ApplicationDigest any        //
	SubmittedAt       *time.Time //
	ReviewedBy        *int       //
	ReviewReasonCode  *string    //
	ReviewedAt        *time.Time //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
