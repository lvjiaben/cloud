// This file defines the local supplier-subject revocation contract.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SupplierSubjectRevocationCreateReq revokes one opaque local binding.
type SupplierSubjectRevocationCreateReq struct {
	g.Meta          `path:"/admin/compliance/suppliers/{providerCode}/subject-revocations" method:"post" tags:"IDC Compliance Local Supplier" summary:"Revoke local supplier subject binding" dc:"Revoke one exact installation-local binding through the signed local compliance gateway. A current active host session is required." permission:"idc:admin:compliance:supplier:subject:revoke"`
	ProviderCode    string `json:"providerCode" in:"path" v:"required|in:tencent_essbasic,nuonuo_invoice" dc:"Local supplier code" eg:"tencent_essbasic"`
	BindingNo       string `json:"bindingNo" v:"required|regex:^lsb_[0-9a-f]{32}$" dc:"Opaque installation-local binding identifier" eg:"lsb_0123456789abcdef0123456789abcdef"`
	ReasonCode      string `json:"reasonCode" v:"required|in:subject_replaced,supplier_authorization_revoked,security_compromise" dc:"Fixed local revocation reason" eg:"supplier_authorization_revoked"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Last read optimistic version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Stable operation key" eg:"supplier-subject-revoke-20260814-0001"`
}

// SupplierSubjectRevocationCreateRes returns the safe post-revocation projection.
type SupplierSubjectRevocationCreateRes struct {
	Item *SupplierConfigurationItem `json:"item" dc:"Updated secret-safe supplier configuration" eg:"{}"`
}
