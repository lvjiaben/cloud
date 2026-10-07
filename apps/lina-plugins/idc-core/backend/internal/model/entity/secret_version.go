// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SecretVersion is the golang structure for table secret_version.
type SecretVersion struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	SecretNo                 string     `json:"secretNo"                 orm:"secret_no"                   description:""`
	Version                  int        `json:"version"                  orm:"version"                     description:""`
	EnvelopeVersion          int        `json:"envelopeVersion"          orm:"envelope_version"            description:"Canonical envelope and AAD format version"`
	CipherAlgorithm          string     `json:"cipherAlgorithm"          orm:"cipher_algorithm"            description:""`
	PayloadNonce             string     `json:"payloadNonce"             orm:"payload_nonce"               description:"Canonical hexadecimal encoding of the random 96-bit AES-GCM payload nonce"`
	Ciphertext               string     `json:"ciphertext"               orm:"ciphertext"                  description:"Canonical hexadecimal authenticated ciphertext containing no plaintext key material"`
	RequestFingerprint       string     `json:"requestFingerprint"       orm:"request_fingerprint"         description:"DEK-keyed HMAC digest used to reject changed plaintext under one idempotency key"`
	IdempotencyKeyDigest     string     `json:"idempotencyKeyDigest"     orm:"idempotency_key_digest"      description:"SHA-256 digest of the caller-supplied idempotency key"`
	RequestOperation         string     `json:"requestOperation"         orm:"request_operation"           description:"Authenticated write operation discriminator: create, generate_u256, or update"`
	RequestParentVersion     int        `json:"requestParentVersion"     orm:"request_parent_version"      description:"Authenticated immutable version fence observed before this write"`
	RequestExpectedUpdatedAt *time.Time `json:"requestExpectedUpdatedAt" orm:"request_expected_updated_at" description:"Authenticated optimistic timestamp fence supplied for an update"`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
}
