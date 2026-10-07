// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceOrderCancellation is the golang structure for table commerce_order_cancellation.
type CommerceOrderCancellation struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	CancellationNo            string     `json:"cancellationNo"            orm:"cancellation_no"              description:""`
	OrderNo                   string     `json:"orderNo"                   orm:"order_no"                     description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                  description:""`
	SourceCode                string     `json:"sourceCode"                orm:"source_code"                  description:""`
	ReasonCode                string     `json:"reasonCode"                orm:"reason_code"                  description:""`
	RequestDigest             string     `json:"requestDigest"             orm:"request_digest"               description:""`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                  description:""`
	PaymentClosureOutcomeCode *string    `json:"paymentClosureOutcomeCode" orm:"payment_closure_outcome_code" description:""`
	PaymentClosureReference   *string    `json:"paymentClosureReference"   orm:"payment_closure_reference"    description:""`
	PaymentClosureDigest      *string    `json:"paymentClosureDigest"      orm:"payment_closure_digest"       description:""`
	RequestedBy               int        `json:"requestedBy"               orm:"requested_by"                 description:""`
	RequestedAt               *time.Time `json:"requestedAt"               orm:"requested_at"                 description:""`
	CompletedAt               *time.Time `json:"completedAt"               orm:"completed_at"                 description:""`
	Version                   int64      `json:"version"                   orm:"version"                      description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                   description:""`
}
