// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RedLetterCandidate is the golang structure for table red_letter_candidate.
type RedLetterCandidate struct {
	Id                     int64      `json:"id"                     orm:"id"                        description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                 description:""`
	CandidateNo            string     `json:"candidateNo"            orm:"candidate_no"              description:""`
	TaskNo                 string     `json:"taskNo"                 orm:"task_no"                   description:""`
	RefundAllocationNo     string     `json:"refundAllocationNo"     orm:"refund_allocation_no"      description:""`
	TaxInvoiceNo           string     `json:"taxInvoiceNo"           orm:"tax_invoice_no"            description:""`
	TaxInvoiceAllocationNo string     `json:"taxInvoiceAllocationNo" orm:"tax_invoice_allocation_no" description:""`
	TaxInvoiceItemNo       string     `json:"taxInvoiceItemNo"       orm:"tax_invoice_item_no"       description:""`
	RequestedGrossMinor    int64      `json:"requestedGrossMinor"    orm:"requested_gross_minor"     description:""`
	AvailableGrossMinor    int64      `json:"availableGrossMinor"    orm:"available_gross_minor"     description:""`
	AvailableNetMinor      int64      `json:"availableNetMinor"      orm:"available_net_minor"       description:""`
	AvailableTaxMinor      int64      `json:"availableTaxMinor"      orm:"available_tax_minor"       description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"                description:""`
}
