// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerSshPublicKey is the golang structure for table customer_ssh_public_key.
type CustomerSshPublicKey struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	KeyNo                string     `json:"keyNo"                orm:"key_no"                 description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:""`
	Name                 string     `json:"name"                 orm:"name"                   description:""`
	AlgorithmCode        string     `json:"algorithmCode"        orm:"algorithm_code"         description:""`
	PublicKey            string     `json:"publicKey"            orm:"public_key"             description:""`
	FingerprintSha256    string     `json:"fingerprintSha256"    orm:"fingerprint_sha256"     description:""`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:""`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	RevokedAt            *time.Time `json:"revokedAt"            orm:"revoked_at"             description:""`
}
