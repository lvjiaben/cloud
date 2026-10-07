// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditRepaymentAllocation is the golang structure of table plugin_idc_core_credit_repayment_allocation for DAO operations like Where/Data.
type CreditRepaymentAllocation struct {
	g.Meta               `orm:"table:plugin_idc_core_credit_repayment_allocation, do:true"`
	Id                   any        //
	TenantId             any        //
	AllocationNo         any        //
	RepaymentNo          any        //
	CreditAccountNo      any        //
	CustomerNo           any        //
	CurrencyCode         any        //
	StatementNo          *string    //
	ExposureNo           *string    //
	AllocationTypeCode   any        //
	DirectionCode        any        //
	AmountMinor          any        //
	OriginalAllocationNo *string    //
	SourceReference      any        //
	ContentDigest        any        //
	CreatedAt            *time.Time //
}
