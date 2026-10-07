// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterAllocation is the golang structure of table plugin_idc_compliance_red_letter_allocation for DAO operations like Where/Data.
type RedLetterAllocation struct {
	g.Meta                 `orm:"table:plugin_idc_compliance_red_letter_allocation, do:true"`
	Id                     any        //
	TenantId               any        //
	AllocationNo           any        //
	TaskNo                 any        //
	TaxInvoiceNo           any        //
	TaxInvoiceAllocationNo any        //
	RefundAllocationNo     any        //
	GrossAmountMinor       any        //
	NetAmountMinor         any        //
	TaxAmountMinor         any        //
	CreatedAt              *time.Time //
	TaxInvoiceItemNo       any        //
}
