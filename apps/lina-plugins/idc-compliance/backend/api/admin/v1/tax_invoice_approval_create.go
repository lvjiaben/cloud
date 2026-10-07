// This file defines the tax-invoice approval action resource.
package v1

import "github.com/gogf/gf/v2/frame/g"

// TaxInvoiceApprovalCreateReq confirms one provider-supported invoice type.
type TaxInvoiceApprovalCreateReq struct {
	g.Meta          `path:"/admin/compliance/tax-invoices/{taxInvoiceNo}/approvals" method:"post" tags:"IDC Compliance Tax Invoice" summary:"Approve a tax invoice request" dc:"Confirm one type currently supported by the installation provider and move a pending request to waiting for submission. A closed capability gate preserves the pending-review state and returns its blocker." permission:"idc:admin:compliance:tax-invoice:review"`
	TaxInvoiceNo    string `json:"taxInvoiceNo" v:"required|max-length:64" dc:"Stable tax invoice request number" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	InvoiceTypeCode string `json:"invoiceTypeCode" v:"required|max-length:32" dc:"Provider-supported type: vat_electronic_normal, vat_electronic_special, digital_electronic_normal, or digital_electronic_special" eg:"vat_electronic_normal"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed request version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"tax-invoice-approval-0001"`
}

// TaxInvoiceApprovalCreateRes wraps the approval or safe gate blocker.
type TaxInvoiceApprovalCreateRes struct {
	Result *TaxInvoiceManagementActionResult `json:"result" dc:"Approval result and current capability" eg:"{}"`
}
