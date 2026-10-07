// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceLifecycleTask is the golang structure for table service_lifecycle_task.
type ServiceLifecycleTask struct {
	Id                           int64      `json:"id"                           orm:"id"                              description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                       description:""`
	TaskNo                       string     `json:"taskNo"                       orm:"task_no"                         description:""`
	ServiceNo                    string     `json:"serviceNo"                    orm:"service_no"                      description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                     description:""`
	CurrencyCode                 string     `json:"currencyCode"                 orm:"currency_code"                   description:""`
	PolicyNo                     string     `json:"policyNo"                     orm:"policy_no"                       description:""`
	PolicyVersion                int64      `json:"policyVersion"                orm:"policy_version"                  description:""`
	ActionCode                   string     `json:"actionCode"                   orm:"action_code"                     description:""`
	ReminderSequence             *int       `json:"reminderSequence"             orm:"reminder_sequence"               description:""`
	ExpectedServiceVersion       int64      `json:"expectedServiceVersion"       orm:"expected_service_version"        description:""`
	ExpectedEntitlementExpiresAt *time.Time `json:"expectedEntitlementExpiresAt" orm:"expected_entitlement_expires_at" description:""`
	DueAt                        *time.Time `json:"dueAt"                        orm:"due_at"                          description:""`
	StatusCode                   string     `json:"statusCode"                   orm:"status_code"                     description:""`
	BusinessOperationNo          *string    `json:"businessOperationNo"          orm:"business_operation_no"           description:""`
	PreviousRuntimeStatusCode    *string    `json:"previousRuntimeStatusCode"    orm:"previous_runtime_status_code"    description:""`
	LastErrorCode                string     `json:"lastErrorCode"                orm:"last_error_code"                 description:""`
	IdempotencyDigest            string     `json:"idempotencyDigest"            orm:"idempotency_digest"              description:""`
	ContentDigest                string     `json:"contentDigest"                orm:"content_digest"                  description:""`
	ClaimedAt                    *time.Time `json:"claimedAt"                    orm:"claimed_at"                      description:""`
	DispatchedAt                 *time.Time `json:"dispatchedAt"                 orm:"dispatched_at"                   description:""`
	CompletedAt                  *time.Time `json:"completedAt"                  orm:"completed_at"                    description:""`
	Version                      int64      `json:"version"                      orm:"version"                         description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                      description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                      description:""`
	ResidualResources            string     `json:"residualResources"            orm:"residual_resources"              description:"At most 16 bounded non-secret resource kind, opaque number, safe status and stable reason projections that still require reclamation"`
}
