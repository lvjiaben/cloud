// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyCustomerRequest is the golang structure for table privacy_customer_request.
type PrivacyCustomerRequest struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	RequestNo       string     `json:"requestNo"       orm:"request_no"        description:""`
	CustomerNo      string     `json:"customerNo"      orm:"customer_no"       description:""`
	RequestTypeCode string     `json:"requestTypeCode" orm:"request_type_code" description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	HoldReasonCode  string     `json:"holdReasonCode"  orm:"hold_reason_code"  description:""`
	SubmittedAt     *time.Time `json:"submittedAt"     orm:"submitted_at"      description:""`
	CompletedAt     *time.Time `json:"completedAt"     orm:"completed_at"      description:""`
	Version         int64      `json:"version"         orm:"version"           description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
