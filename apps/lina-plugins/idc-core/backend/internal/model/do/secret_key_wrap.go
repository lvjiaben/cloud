// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SecretKeyWrap is the golang structure of table plugin_idc_core_secret_key_wrap for DAO operations like Where/Data.
type SecretKeyWrap struct {
	g.Meta          `orm:"table:plugin_idc_core_secret_key_wrap, do:true"`
	Id              any        //
	TenantId        any        //
	SecretNo        any        //
	SecretVersion   any        //
	EnvelopeVersion any        //
	MasterKeyId     any        // External deployment key identifier; master-key bytes are never persisted
	WrapAlgorithm   any        //
	WrapNonce       any        // Canonical hexadecimal encoding of the random 96-bit AES-GCM wrapper nonce
	WrappedDek      any        // Canonical hexadecimal authenticated 32-byte DEK ciphertext plus the 16-byte GCM tag
	CreatedAt       *time.Time //
}
