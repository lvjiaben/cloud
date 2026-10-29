// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// LoginContactGuard is the golang structure for table login_contact_guard.
type LoginContactGuard struct {
	TenantId           int       `json:"tenantId"           orm:"tenant_id"            description:""`
	UserId             int       `json:"userId"             orm:"user_id"              description:""`
	Revision           int64     `json:"revision"           orm:"revision"             description:""`
	CreatedAt          time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
	RecoveryNo         string    `json:"recoveryNo"         orm:"recovery_no"          description:""`
	CleanupPending     bool      `json:"cleanupPending"     orm:"cleanup_pending"      description:""`
	RecoveryAt         time.Time `json:"recoveryAt"         orm:"recovery_at"          description:""`
	RecoveryRevision   int64     `json:"recoveryRevision"   orm:"recovery_revision"    description:""`
	CleanupTokenDigest string    `json:"cleanupTokenDigest" orm:"cleanup_token_digest" description:""`
}
