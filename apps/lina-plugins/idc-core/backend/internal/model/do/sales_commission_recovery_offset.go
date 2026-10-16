// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionRecoveryOffset is the golang structure of table plugin_idc_core_sales_commission_recovery_offset for DAO operations like Where/Data.
type SalesCommissionRecoveryOffset struct {
	g.Meta       `orm:"table:plugin_idc_core_sales_commission_recovery_offset, do:true"`
	Id           any        //
	TenantId     any        //
	OffsetNo     any        //
	RecoveryNo   any        //
	CommissionNo any        //
	AccountNo    any        //
	AmountMinor  any        //
	EntryNo      any        //
	CreatedAt    *time.Time //
}
