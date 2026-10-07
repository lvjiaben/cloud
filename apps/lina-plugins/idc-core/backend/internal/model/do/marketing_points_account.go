// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPointsAccount is the golang structure of table plugin_idc_core_marketing_points_account for DAO operations like Where/Data.
type MarketingPointsAccount struct {
	g.Meta          `orm:"table:plugin_idc_core_marketing_points_account, do:true"`
	Id              any        //
	TenantId        any        //
	CustomerNo      any        //
	AvailablePoints any        //
	FrozenPoints    any        //
	DebtPoints      any        //
	Version         any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
