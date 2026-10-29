// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NativeServerWriteFence is the golang structure for table native_server_write_fence.
type NativeServerWriteFence struct {
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	TaskNo                     string     `json:"taskNo"                     orm:"task_no"                      description:""`
	OperationNo                string     `json:"operationNo"                orm:"operation_no"                 description:""`
	EntitlementNo              string     `json:"entitlementNo"              orm:"entitlement_no"               description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                  description:""`
	ExpectedEntitlementVersion int64      `json:"expectedEntitlementVersion" orm:"expected_entitlement_version" description:""`
	ActionCode                 string     `json:"actionCode"                 orm:"action_code"                  description:""`
	RequestDigest              string     `json:"requestDigest"              orm:"request_digest"               description:""`
	StatusCode                 string     `json:"statusCode"                 orm:"status_code"                  description:""`
	EvidenceDigest             string     `json:"evidenceDigest"             orm:"evidence_digest"              description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
	UpdatedAt                  *time.Time `json:"updatedAt"                  orm:"updated_at"                   description:""`
	CompletedAt                *time.Time `json:"completedAt"                orm:"completed_at"                 description:""`
}
