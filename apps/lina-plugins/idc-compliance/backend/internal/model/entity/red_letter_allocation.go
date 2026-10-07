// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RedLetterAllocation is the golang structure for table red_letter_allocation.
type RedLetterAllocation struct {
	Id                     int64      `json:"id"                     orm:"id"                        description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                 description:""`
	AllocationNo           string     `json:"allocationNo"           orm:"allocation_no"             description:""`
	TaskNo                 string     `json:"taskNo"                 orm:"task_no"                   description:""`
	TaxInvoiceNo           string     `json:"taxInvoiceNo"           orm:"tax_invoice_no"            description:""`
	TaxInvoiceAllocationNo string     `json:"taxInvoiceAllocationNo" orm:"tax_invoice_allocation_no" description:""`
	RefundAllocationNo     string     `json:"refundAllocationNo"     orm:"refund_allocation_no"      description:""`
	GrossAmountMinor       int64      `json:"grossAmountMinor"       orm:"gross_amount_minor"        description:""`
	NetAmountMinor         int64      `json:"netAmountMinor"         orm:"net_amount_minor"          description:""`
	TaxAmountMinor         int64      `json:"taxAmountMinor"         orm:"tax_amount_minor"          description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"                description:""`
	TaxInvoiceItemNo       string     `json:"taxInvoiceItemNo"       orm:"tax_invoice_item_no"       description:""`
}
