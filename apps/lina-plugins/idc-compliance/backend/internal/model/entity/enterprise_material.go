// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EnterpriseMaterial is the golang structure for table enterprise_material.
type EnterpriseMaterial struct {
	Id                     int64      `json:"id"                     orm:"id"                        description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                 description:""`
	MaterialNo             string     `json:"materialNo"             orm:"material_no"               description:""`
	CaseNo                 string     `json:"caseNo"                 orm:"case_no"                   description:""`
	CaseSubjectTypeCode    string     `json:"caseSubjectTypeCode"    orm:"case_subject_type_code"    description:""`
	MaterialTypeCode       string     `json:"materialTypeCode"       orm:"material_type_code"        description:""`
	SequenceNo             int        `json:"sequenceNo"             orm:"sequence_no"               description:""`
	PrivateObjectRef       string     `json:"privateObjectRef"       orm:"private_object_ref"        description:""`
	ContentSha256          string     `json:"contentSha256"          orm:"content_sha256"            description:"SHA-256 of the encrypted storage envelope, never a digest derived directly from document plaintext"`
	MediaType              string     `json:"mediaType"              orm:"media_type"                description:""`
	ByteSize               int64      `json:"byteSize"               orm:"byte_size"                 description:""`
	EncryptionStateCode    string     `json:"encryptionStateCode"    orm:"encryption_state_code"     description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"               description:""`
	RetentionUntil         *time.Time `json:"retentionUntil"         orm:"retention_until"           description:""`
	LegalHold              bool       `json:"legalHold"              orm:"legal_hold"                description:""`
	SupersededByMaterialNo *string    `json:"supersededByMaterialNo" orm:"superseded_by_material_no" description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"                description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"                description:""`
	EncryptionKeyNo        string     `json:"encryptionKeyNo"        orm:"encryption_key_no"         description:"Low-entropy keyring generation code; the secretcap handle remains in the keyring table only"`
	StoredByteSize         int64      `json:"storedByteSize"         orm:"stored_byte_size"          description:"Encrypted envelope byte size confirmed against private object storage metadata"`
	IdempotencyKeyDigest   string     `json:"idempotencyKeyDigest"   orm:"idempotency_key_digest"    description:""`
	RequestFingerprint     string     `json:"requestFingerprint"     orm:"request_fingerprint"       description:""`
}
