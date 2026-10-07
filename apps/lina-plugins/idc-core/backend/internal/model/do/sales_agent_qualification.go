// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentQualification is the golang structure of table plugin_idc_core_sales_agent_qualification for DAO operations like Where/Data.
type SalesAgentQualification struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_agent_qualification, do:true"`
	Id                  any        //
	TenantId            any        //
	QualificationNo     any        //
	CustomerNo          any        //
	LevelNo             any        //
	SourceApplicationNo *string    //
	StatusCode          any        //
	EffectiveAt         *time.Time //
	ExpiresAt           *time.Time //
	Version             any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
