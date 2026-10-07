// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupplierConfiguration is the golang structure of table plugin_idc_compliance_supplier_configuration for DAO operations like Where/Data.
type SupplierConfiguration struct {
	g.Meta                  `orm:"table:plugin_idc_compliance_supplier_configuration, do:true"`
	Id                      any        //
	TenantId                any        //
	ConfigurationNo         any        //
	ProviderCode            any        //
	CredentialStatusCode    any        // Credential lifecycle only; independent from supplier subject binding state
	CredentialSecretRef     *string    // Sensitive secretcap routing reference for one provider-neutral credential envelope; plaintext and ciphertext are forbidden
	CredentialGeneration    any        //
	CapabilityEvidenceCode  any        // Reviewed provider-contract gate; official_public_contract is valid only for Tencent ESS Basic
	AdapterEnabled          any        // True only for an active Tencent local subject backed by the reviewed public API contract
	LastMutationDigest      *string    //
	MutationOperationKey    *string    //
	MutationSecretUpdatedAt *time.Time //
	Version                 any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
	SubjectStatusCode       any        //
	SubjectBindingNo        *string    // Opaque installation-local gateway binding identifier; supplier subject plaintext is forbidden
	LastOperationKindCode   any        //
	LastOperationResultCode any        //
	LastOperationNo         *string    // Opaque plugin-local operation identifier; provider payloads and subject identifiers are forbidden
	LastOperationAt         *time.Time //
	SubjectProofDigest      *string    // Digest of the current server-only single-use proof; raw proof tokens are forbidden
	SubjectProofExpiresAt   *time.Time //
}
