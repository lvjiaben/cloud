// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SecretKeyWrap is the golang structure for table secret_key_wrap.
type SecretKeyWrap struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:""`
	SecretNo        string     `json:"secretNo"        orm:"secret_no"        description:""`
	SecretVersion   int        `json:"secretVersion"   orm:"secret_version"   description:""`
	EnvelopeVersion int        `json:"envelopeVersion" orm:"envelope_version" description:""`
	MasterKeyId     string     `json:"masterKeyId"     orm:"master_key_id"    description:"External deployment key identifier; master-key bytes are never persisted"`
	WrapAlgorithm   string     `json:"wrapAlgorithm"   orm:"wrap_algorithm"   description:""`
	WrapNonce       string     `json:"wrapNonce"       orm:"wrap_nonce"       description:"Canonical hexadecimal encoding of the random 96-bit AES-GCM wrapper nonce"`
	WrappedDek      string     `json:"wrappedDek"      orm:"wrapped_dek"      description:"Canonical hexadecimal authenticated 32-byte DEK ciphertext plus the 16-byte GCM tag"`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
}
