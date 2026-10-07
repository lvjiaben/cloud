// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditRepaymentAllocation is the golang structure for table credit_repayment_allocation.
type CreditRepaymentAllocation struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	AllocationNo         string     `json:"allocationNo"         orm:"allocation_no"          description:""`
	RepaymentNo          string     `json:"repaymentNo"          orm:"repayment_no"           description:""`
	CreditAccountNo      string     `json:"creditAccountNo"      orm:"credit_account_no"      description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"          description:""`
	StatementNo          *string    `json:"statementNo"          orm:"statement_no"           description:""`
	ExposureNo           *string    `json:"exposureNo"           orm:"exposure_no"            description:""`
	AllocationTypeCode   string     `json:"allocationTypeCode"   orm:"allocation_type_code"   description:""`
	DirectionCode        string     `json:"directionCode"        orm:"direction_code"         description:""`
	AmountMinor          int64      `json:"amountMinor"          orm:"amount_minor"           description:""`
	OriginalAllocationNo *string    `json:"originalAllocationNo" orm:"original_allocation_no" description:""`
	SourceReference      string     `json:"sourceReference"      orm:"source_reference"       description:""`
	ContentDigest        string     `json:"contentDigest"        orm:"content_digest"         description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
}
