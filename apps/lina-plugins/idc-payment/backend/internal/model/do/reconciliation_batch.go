// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationBatch is the golang structure of table plugin_idc_payment_reconciliation_batch for DAO operations like Where/Data.
type ReconciliationBatch struct {
	g.Meta           `orm:"table:plugin_idc_payment_reconciliation_batch, do:true"`
	Id               any        //
	BatchNo          any        //
	ChannelNo        any        //
	BusinessDate     any        //
	SourceDigest     any        //
	StatusCode       any        //
	TotalCount       any        //
	DifferenceCount  any        //
	CreatedAt        *time.Time //
	FinishedAt       *time.Time //
	DailyScopeKey    any        //
	MerchantTimezone any        //
	SourceTypeCode   any        //
	CapabilityCode   any        //
	PaymentCount     any        //
	RefundCount      any        //
	ResultUnknown    any        //
	FailureCode      any        //
	UpdatedAt        *time.Time //
	Version          any        //
}
