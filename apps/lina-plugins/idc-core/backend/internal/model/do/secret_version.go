// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SecretVersion is the golang structure of table plugin_idc_core_secret_version for DAO operations like Where/Data.
type SecretVersion struct {
	g.Meta                   `orm:"table:plugin_idc_core_secret_version, do:true"`
	Id                       any        //
	TenantId                 any        //
	SecretNo                 any        //
	Version                  any        //
	EnvelopeVersion          any        // Canonical envelope and AAD format version
	CipherAlgorithm          any        //
	PayloadNonce             any        // Canonical hexadecimal encoding of the random 96-bit AES-GCM payload nonce
	Ciphertext               any        // Canonical hexadecimal authenticated ciphertext containing no plaintext key material
	RequestFingerprint       any        // DEK-keyed HMAC digest used to reject changed plaintext under one idempotency key
	IdempotencyKeyDigest     any        // SHA-256 digest of the caller-supplied idempotency key
	RequestOperation         any        // Authenticated write operation discriminator: create, generate_u256, or update
	RequestParentVersion     any        // Authenticated immutable version fence observed before this write
	RequestExpectedUpdatedAt *time.Time // Authenticated optimistic timestamp fence supplied for an update
	CreatedAt                *time.Time //
}
