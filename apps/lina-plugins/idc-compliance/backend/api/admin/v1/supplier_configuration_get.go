// This file defines the safe administration projection for one supplier configuration.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SupplierConfigurationGetReq reads one fixed installation-local supplier target.
type SupplierConfigurationGetReq struct {
	g.Meta       `path:"/admin/compliance/suppliers/{providerCode}" method:"get" tags:"IDC Compliance Local Supplier" summary:"Get local merchant supplier configuration" dc:"Return one installation-local Tencent ESS Basic or Nuonuo configuration without secret references, credentials, ciphertext, external identifiers, or provider payloads." permission:"idc:admin:compliance:supplier:query"`
	ProviderCode string `json:"providerCode" in:"path" v:"required|in:tencent_essbasic,nuonuo_invoice" dc:"Local supplier code: tencent_essbasic or nuonuo_invoice" eg:"tencent_essbasic"`
}

// SupplierConfigurationGetRes returns one safe local configuration projection.
type SupplierConfigurationGetRes struct {
	Item *SupplierConfigurationItem `json:"item" dc:"Secret-safe installation-local supplier configuration" eg:"{}"`
}

// SupplierConfigurationItem never exposes a secretcap handle or credential bytes.
type SupplierConfigurationItem struct {
	ProviderCode           string `json:"providerCode" dc:"Local supplier code" eg:"tencent_essbasic"`
	CredentialStatus       string `json:"credentialStatus" dc:"Credential state: unconfigured, updating, configured, or retryable_failed" eg:"configured"`
	CredentialConfigured   bool   `json:"credentialConfigured" dc:"Whether the credential envelope resolves to active secret metadata" eg:"true"`
	CredentialMaskedTail   string `json:"credentialMaskedTail" dc:"Display-only credential suffix; empty when unavailable" eg:"A001"`
	CredentialGeneration   int64  `json:"credentialGeneration" dc:"Committed local credential generation" eg:"1"`
	SubjectStatus          string `json:"subjectStatus" dc:"Subject state: unbound, binding, active, revoked, reopening, or retryable_failed" eg:"unbound"`
	SubjectBindingNo       string `json:"subjectBindingNo" dc:"Opaque installation-local binding identifier; empty before binding" eg:"lsb_0123456789abcdef0123456789abcdef"`
	LastOperationKind      string `json:"lastOperationKind" dc:"Last subject operation kind" eg:"subject_revoke"`
	LastOperationResult    string `json:"lastOperationResult" dc:"Last subject operation result: none, pending, succeeded, or retryable_failed" eg:"succeeded"`
	LastOperationNo        string `json:"lastOperationNo" dc:"Opaque installation-local operation identifier" eg:"lsi_0123456789abcdef0123456789abcdef"`
	LastOperationAt        *int64 `json:"lastOperationAt" dc:"Last subject operation Unix timestamp in milliseconds" eg:"1786464000000"`
	CapabilityEvidenceCode string `json:"capabilityEvidenceCode" dc:"Reviewed provider gate: external_confirmation_required or Tencent-only official_public_contract" eg:"official_public_contract"`
	AdapterEnabled         bool   `json:"adapterEnabled" dc:"Whether the reviewed adapter has an active installation-local subject" eg:"true"`
	Version                int64  `json:"version" dc:"Optimistic concurrency version; zero before first configuration" eg:"2"`
	UpdatedAt              *int64 `json:"updatedAt" dc:"Latest configuration update Unix timestamp in milliseconds" eg:"1786464000000"`
}
