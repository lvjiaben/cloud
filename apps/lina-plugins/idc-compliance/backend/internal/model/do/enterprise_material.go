// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EnterpriseMaterial is the golang structure of table plugin_idc_compliance_enterprise_material for DAO operations like Where/Data.
type EnterpriseMaterial struct {
	g.Meta                 `orm:"table:plugin_idc_compliance_enterprise_material, do:true"`
	Id                     any        //
	TenantId               any        //
	MaterialNo             any        //
	CaseNo                 any        //
	CaseSubjectTypeCode    any        //
	MaterialTypeCode       any        //
	SequenceNo             any        //
	PrivateObjectRef       any        //
	ContentSha256          any        // SHA-256 of the encrypted storage envelope, never a digest derived directly from document plaintext
	MediaType              any        //
	ByteSize               any        //
	EncryptionStateCode    any        //
	StatusCode             any        //
	RetentionUntil         *time.Time //
	LegalHold              any        //
	SupersededByMaterialNo *string    //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
	EncryptionKeyNo        any        // Low-entropy keyring generation code; the secretcap handle remains in the keyring table only
	StoredByteSize         any        // Encrypted envelope byte size confirmed against private object storage metadata
	IdempotencyKeyDigest   any        //
	RequestFingerprint     any        //
}
