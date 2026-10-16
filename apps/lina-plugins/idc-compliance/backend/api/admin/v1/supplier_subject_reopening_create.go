// This file defines the external-confirmation gate for reopening a supplier subject.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SupplierSubjectReopeningCreateReq starts a distinct reauthorization flow.
type SupplierSubjectReopeningCreateReq struct {
	g.Meta       `path:"/admin/compliance/suppliers/{providerCode}/subject-reopenings" method:"post" tags:"IDC Compliance Local Supplier" summary:"Start local supplier subject reopening" dc:"Require exact permission and a current active session. Until genuine supplier recognition evidence is archived, return external-confirmation-required without creating a proof or changing state." permission:"idc:admin:compliance:supplier:subject:reopen"`
	ProviderCode string `json:"providerCode" in:"path" v:"required|in:tencent_essbasic,nuonuo_invoice" dc:"Local supplier code" eg:"nuonuo_invoice"`
}

// SupplierSubjectReopeningCreateRes is empty because the external evidence gate is closed.
type SupplierSubjectReopeningCreateRes struct{}
