// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaxInvoiceAllocation is the golang structure for table tax_invoice_allocation.
type TaxInvoiceAllocation struct {
	Id                    int64      `json:"id"                    orm:"id"                       description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"                description:""`
	AllocationNo          string     `json:"allocationNo"          orm:"allocation_no"            description:""`
	TaxInvoiceNo          string     `json:"taxInvoiceNo"          orm:"tax_invoice_no"           description:""`
	BillingInvoiceNo      string     `json:"billingInvoiceNo"      orm:"billing_invoice_no"       description:"Opaque reference to a core receivable; no cross-plugin foreign key is permitted"`
	OrderNo               string     `json:"orderNo"               orm:"order_no"                 description:""`
	AllocatedGrossMinor   int64      `json:"allocatedGrossMinor"   orm:"allocated_gross_minor"    description:""`
	AllocatedNetMinor     int64      `json:"allocatedNetMinor"     orm:"allocated_net_minor"      description:""`
	AllocatedTaxMinor     int64      `json:"allocatedTaxMinor"     orm:"allocated_tax_minor"      description:""`
	RefundedGrossMinor    int64      `json:"refundedGrossMinor"    orm:"refunded_gross_minor"     description:""`
	RedLetteredGrossMinor int64      `json:"redLetteredGrossMinor" orm:"red_lettered_gross_minor" description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"               description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"               description:""`
}
