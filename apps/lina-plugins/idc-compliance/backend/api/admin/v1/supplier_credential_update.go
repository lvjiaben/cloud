// This file defines the merchant-owned supplier credential update contract.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SupplierCredentialUpdateReq consumes one provider-neutral secret envelope.
type SupplierCredentialUpdateReq struct {
	g.Meta             `path:"/admin/compliance/suppliers/{providerCode}/credential" method:"put" tags:"IDC Compliance Local Supplier" summary:"Update local supplier credential" dc:"Create, resume, or replace one installation-local merchant credential through a fail-closed audited transition. A current active host session is required. Subject selection is a separate operation and the credential is never returned." permission:"idc:admin:compliance:supplier:credential:write"`
	ProviderCode       string `json:"providerCode" in:"path" v:"required|in:tencent_essbasic,nuonuo_invoice" dc:"Local supplier code" eg:"nuonuo_invoice"`
	CredentialEnvelope string `json:"credentialEnvelope" v:"required|min-length:16|max-length:16384" dc:"Provider-neutral credential envelope consumed by secretcap and never returned" eg:"opaque-merchant-owned-credential-envelope"`
	ExpectedVersion    int64  `json:"expectedVersion" v:"min:0" dc:"Last read optimistic version; use zero only for first configuration" eg:"0"`
	IdempotencyKey     string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Stable operation key" eg:"supplier-credential-20260814-0001"`
}

// SupplierCredentialUpdateRes returns the committed safe configuration projection.
type SupplierCredentialUpdateRes struct {
	Item *SupplierConfigurationItem `json:"item" dc:"Committed secret-safe supplier configuration" eg:"{}"`
}
