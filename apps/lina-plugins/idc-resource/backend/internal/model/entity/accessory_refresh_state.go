// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AccessoryRefreshState is the golang structure for table accessory_refresh_state.
type AccessoryRefreshState struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	MappingNo        string     `json:"mappingNo"        orm:"mapping_no"         description:""`
	NextAttemptAt    *time.Time `json:"nextAttemptAt"    orm:"next_attempt_at"    description:""`
	AttemptCount     int        `json:"attemptCount"     orm:"attempt_count"      description:""`
	LeaseTokenDigest string     `json:"leaseTokenDigest" orm:"lease_token_digest" description:""`
	LeaseExpiresAt   *time.Time `json:"leaseExpiresAt"   orm:"lease_expires_at"   description:""`
	FailureCode      string     `json:"failureCode"      orm:"failure_code"       description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"         description:""`
}
