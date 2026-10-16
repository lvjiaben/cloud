// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommissionRefundReversal is the golang structure of table plugin_idc_core_commission_refund_reversal for DAO operations like Where/Data.
type CommissionRefundReversal struct {
	g.Meta        `orm:"table:plugin_idc_core_commission_refund_reversal, do:true"`
	Id            any        //
	TenantId      any        //
	ReversalNo    any        //
	CommissionNo  any        //
	RefundNo      any        //
	SourceEventNo any        //
	AmountMinor   any        //
	CreatedAt     *time.Time //
}
