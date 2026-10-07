// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingBalanceSnapshot is the golang structure of table plugin_idc_core_billing_balance_snapshot for DAO operations like Where/Data.
type BillingBalanceSnapshot struct {
	g.Meta              `orm:"table:plugin_idc_core_billing_balance_snapshot, do:true"`
	Id                  any        //
	TenantId            any        //
	SnapshotNo          any        //
	AccountNo           any        //
	CurrencyCode        any        //
	NormalBalanceCode   any        //
	TransactionNo       any        //
	SequenceNo          any        //
	PreviousSnapshotNo  *string    //
	PreviousSequenceNo  *int64     //
	OpeningBalanceMinor any        //
	DebitAmountMinor    any        //
	CreditAmountMinor   any        //
	ClosingBalanceMinor any        //
	ContentDigest       any        //
	CreatedAt           *time.Time //
}
