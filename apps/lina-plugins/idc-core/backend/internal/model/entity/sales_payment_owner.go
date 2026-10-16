// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesPaymentOwner is the golang structure for table sales_payment_owner.
type SalesPaymentOwner struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	InvoiceNo              string     `json:"invoiceNo"              orm:"invoice_no"               description:""`
	BeneficiaryTypeCode    string     `json:"beneficiaryTypeCode"    orm:"beneficiary_type_code"    description:""`
	BeneficiaryReference   string     `json:"beneficiaryReference"   orm:"beneficiary_reference"    description:""`
	SalesAssignmentNo      *string    `json:"salesAssignmentNo"      orm:"sales_assignment_no"      description:""`
	AffiliateAttributionNo *string    `json:"affiliateAttributionNo" orm:"affiliate_attribution_no" description:""`
	ManualReferrerId       *int64     `json:"manualReferrerId"       orm:"manual_referrer_id"       description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
}
