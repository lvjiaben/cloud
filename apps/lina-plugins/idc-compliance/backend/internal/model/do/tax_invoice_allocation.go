// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceAllocation is the golang structure of table plugin_idc_compliance_tax_invoice_allocation for DAO operations like Where/Data.
type TaxInvoiceAllocation struct {
	g.Meta                `orm:"table:plugin_idc_compliance_tax_invoice_allocation, do:true"`
	Id                    any        //
	TenantId              any        //
	AllocationNo          any        //
	TaxInvoiceNo          any        //
	BillingInvoiceNo      any        // Opaque reference to a core receivable; no cross-plugin foreign key is permitted
	OrderNo               any        //
	AllocatedGrossMinor   any        //
	AllocatedNetMinor     any        //
	AllocatedTaxMinor     any        //
	RefundedGrossMinor    any        //
	RedLetteredGrossMinor any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
}
