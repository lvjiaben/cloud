// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// IdempotencyRecord is the golang structure for table idempotency_record.
type IdempotencyRecord struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	Scope                string     `json:"scope"                orm:"scope"                  description:"Stable command or endpoint idempotency namespace"`
	CallerScopeDigest    string     `json:"callerScopeDigest"    orm:"caller_scope_digest"    description:"Digest of the normalized customer, staff, channel, or system caller scope"`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:"SHA-256 digest of the caller idempotency key"`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:"SHA-256 digest of canonical request semantics"`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	LockOwner            string     `json:"lockOwner"            orm:"lock_owner"             description:""`
	LockExpiresAt        *time.Time `json:"lockExpiresAt"        orm:"lock_expires_at"        description:""`
	ResourceType         string     `json:"resourceType"         orm:"resource_type"          description:""`
	ResourceNo           string     `json:"resourceNo"           orm:"resource_no"            description:""`
	ResponseStatusCode   int        `json:"responseStatusCode"   orm:"response_status_code"   description:""`
	ResponseSnapshot     string     `json:"responseSnapshot"     orm:"response_snapshot"      description:"Redacted normalized response used for safe replay"`
	ExpiresAt            *time.Time `json:"expiresAt"            orm:"expires_at"             description:"Earliest time at which retention cleanup may evaluate the record"`
	CompletedAt          *time.Time `json:"completedAt"          orm:"completed_at"           description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
