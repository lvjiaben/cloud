// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RecentAuthConsumption is the golang structure for table recent_auth_consumption.
type RecentAuthConsumption struct {
	Id            int64      `json:"id"            orm:"id"             description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	NonceDigest   string     `json:"nonceDigest"   orm:"nonce_digest"   description:"Domain-separated SHA-256 digest of a random proof nonce; raw nonce and proof are never persisted"`
	PurposeCode   string     `json:"purposeCode"   orm:"purpose_code"   description:""`
	TargetDigest  string     `json:"targetDigest"  orm:"target_digest"  description:"Digest of the immutable dangerous-operation target or preview"`
	SessionDigest string     `json:"sessionDigest" orm:"session_digest" description:"Digest binding to the host session without persisting its raw identifier"`
	UserId        int        `json:"userId"        orm:"user_id"        description:""`
	LoginAt       *time.Time `json:"loginAt"       orm:"login_at"       description:""`
	IssuedAt      *time.Time `json:"issuedAt"      orm:"issued_at"      description:""`
	ExpiresAt     *time.Time `json:"expiresAt"     orm:"expires_at"     description:""`
	ConsumedAt    *time.Time `json:"consumedAt"    orm:"consumed_at"    description:""`
}
