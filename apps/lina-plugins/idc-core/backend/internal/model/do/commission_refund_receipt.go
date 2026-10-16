// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommissionRefundReceipt is the golang structure of table plugin_idc_core_commission_refund_receipt for DAO operations like Where/Data.
type CommissionRefundReceipt struct {
	g.Meta              `orm:"table:plugin_idc_core_commission_refund_receipt, do:true"`
	Id                  any        //
	TenantId            any        //
	EventNo             any        //
	RefundNo            any        //
	PayloadDigest       any        //
	ReversalCount       any        //
	ReversedAmountMinor any        //
	CreatedAt           *time.Time //
}
