// This file defines the external-confirmation gate for starting subject binding.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SupplierSubjectBindingCreateReq starts supplier-owned subject recognition.
type SupplierSubjectBindingCreateReq struct {
	g.Meta       `path:"/admin/compliance/suppliers/{providerCode}/subject-bindings" method:"post" tags:"IDC Compliance Local Supplier" summary:"Start local supplier subject binding" dc:"Require exact permission and a current active session. Until genuine supplier recognition evidence is archived, return external-confirmation-required without creating a proof or changing state." permission:"idc:admin:compliance:supplier:subject:bind"`
	ProviderCode string `json:"providerCode" in:"path" v:"required|in:tencent_essbasic,nuonuo_invoice" dc:"Local supplier code" eg:"tencent_essbasic"`
}

// SupplierSubjectBindingCreateRes is empty because the external evidence gate is closed.
type SupplierSubjectBindingCreateRes struct{}
