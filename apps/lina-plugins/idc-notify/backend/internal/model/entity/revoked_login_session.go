// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RevokedLoginSession is the golang structure for table revoked_login_session.
type RevokedLoginSession struct {
	TenantId      int       `json:"tenantId"      orm:"tenant_id"      description:""`
	UserId        int       `json:"userId"        orm:"user_id"        description:""`
	SessionDigest string    `json:"sessionDigest" orm:"session_digest" description:""`
	SessionId     string    `json:"sessionId"     orm:"session_id"     description:""`
	RecoveryNo    string    `json:"recoveryNo"    orm:"recovery_no"    description:""`
	CleanupDone   bool      `json:"cleanupDone"   orm:"cleanup_done"   description:""`
	CreatedAt     time.Time `json:"createdAt"     orm:"created_at"     description:""`
	UpdatedAt     time.Time `json:"updatedAt"     orm:"updated_at"     description:""`
}
