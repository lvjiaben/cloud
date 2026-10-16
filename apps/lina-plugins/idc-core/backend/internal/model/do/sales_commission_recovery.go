// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionRecovery is the golang structure of table plugin_idc_core_sales_commission_recovery for DAO operations like Where/Data.
type SalesCommissionRecovery struct {
	g.Meta       `orm:"table:plugin_idc_core_sales_commission_recovery, do:true"`
	Id           any        //
	TenantId     any        //
	RecoveryNo   any        //
	AccountNo    any        //
	CommissionNo any        //
	RefundNo     any        //
	ReversalNo   any        //
	AmountMinor  any        //
	CreatedAt    *time.Time //
}
