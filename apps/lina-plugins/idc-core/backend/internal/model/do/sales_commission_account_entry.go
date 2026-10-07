// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionAccountEntry is the golang structure of table plugin_idc_core_sales_commission_account_entry for DAO operations like Where/Data.
type SalesCommissionAccountEntry struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_commission_account_entry, do:true"`
	Id                  any        //
	TenantId            any        //
	EntryNo             any        //
	AccountNo           any        //
	CurrencyCode        any        //
	EntrySequence       any        //
	EntryTypeCode       any        //
	SourceTypeCode      any        //
	SourceReference     any        //
	SourceDigest        any        //
	DeltaCreditedMinor  any        //
	DeltaPendingMinor   any        //
	DeltaAvailableMinor any        //
	DeltaFrozenMinor    any        //
	DeltaPaidMinor      any        //
	DeltaReversedMinor  any        //
	AfterCreditedMinor  any        //
	AfterPendingMinor   any        //
	AfterAvailableMinor any        //
	AfterFrozenMinor    any        //
	AfterPaidMinor      any        //
	AfterReversedMinor  any        //
	OccurredAt          *time.Time //
}
