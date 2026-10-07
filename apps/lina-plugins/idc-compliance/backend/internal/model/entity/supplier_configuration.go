// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupplierConfiguration is the golang structure for table supplier_configuration.
type SupplierConfiguration struct {
	Id                      int64      `json:"id"                      orm:"id"                         description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                  description:""`
	ConfigurationNo         string     `json:"configurationNo"         orm:"configuration_no"           description:""`
	ProviderCode            string     `json:"providerCode"            orm:"provider_code"              description:""`
	CredentialStatusCode    string     `json:"credentialStatusCode"    orm:"credential_status_code"     description:"Credential lifecycle only; independent from supplier subject binding state"`
	CredentialSecretRef     *string    `json:"credentialSecretRef"     orm:"credential_secret_ref"      description:"Sensitive secretcap routing reference for one provider-neutral credential envelope; plaintext and ciphertext are forbidden"`
	CredentialGeneration    int64      `json:"credentialGeneration"    orm:"credential_generation"      description:""`
	CapabilityEvidenceCode  string     `json:"capabilityEvidenceCode"  orm:"capability_evidence_code"   description:"Reviewed provider-contract gate; official_public_contract is valid only for Tencent ESS Basic"`
	AdapterEnabled          bool       `json:"adapterEnabled"          orm:"adapter_enabled"            description:"True only for an active Tencent local subject backed by the reviewed public API contract"`
	LastMutationDigest      *string    `json:"lastMutationDigest"      orm:"last_mutation_digest"       description:""`
	MutationOperationKey    *string    `json:"mutationOperationKey"    orm:"mutation_operation_key"     description:""`
	MutationSecretUpdatedAt *time.Time `json:"mutationSecretUpdatedAt" orm:"mutation_secret_updated_at" description:""`
	Version                 int64      `json:"version"                 orm:"version"                    description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                 description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                 description:""`
	SubjectStatusCode       string     `json:"subjectStatusCode"       orm:"subject_status_code"        description:""`
	SubjectBindingNo        *string    `json:"subjectBindingNo"        orm:"subject_binding_no"         description:"Opaque installation-local gateway binding identifier; supplier subject plaintext is forbidden"`
	LastOperationKindCode   string     `json:"lastOperationKindCode"   orm:"last_operation_kind_code"   description:""`
	LastOperationResultCode string     `json:"lastOperationResultCode" orm:"last_operation_result_code" description:""`
	LastOperationNo         *string    `json:"lastOperationNo"         orm:"last_operation_no"          description:"Opaque plugin-local operation identifier; provider payloads and subject identifiers are forbidden"`
	LastOperationAt         *time.Time `json:"lastOperationAt"         orm:"last_operation_at"          description:""`
	SubjectProofDigest      *string    `json:"subjectProofDigest"      orm:"subject_proof_digest"       description:"Digest of the current server-only single-use proof; raw proof tokens are forbidden"`
	SubjectProofExpiresAt   *time.Time `json:"subjectProofExpiresAt"   orm:"subject_proof_expires_at"   description:""`
}
