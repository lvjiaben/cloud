// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationChannelHealth is the golang structure of table plugin_idc_payment_reconciliation_channel_health for DAO operations like Where/Data.
type ReconciliationChannelHealth struct {
	g.Meta                  `orm:"table:plugin_idc_payment_reconciliation_channel_health, do:true"`
	Id                      any        //
	ChannelNo               any        //
	CapabilityCode          any        //
	HealthCode              any        //
	LatestBatchNo           any        //
	LatestBusinessDate      any        //
	UnresolvedCount         any        //
	ConsecutiveFailureCount any        //
	ResultUnknown           any        //
	FailureCode             any        //
	Version                 any        //
	UpdatedAt               *time.Time //
}
