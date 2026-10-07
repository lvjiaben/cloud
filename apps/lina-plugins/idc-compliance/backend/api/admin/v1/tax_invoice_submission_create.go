// This file defines the local tax-invoice submission action resource.
package v1

import "github.com/gogf/gf/v2/frame/g"

// TaxInvoiceSubmissionCreateReq queues one approved request locally.
type TaxInvoiceSubmissionCreateReq struct {
	g.Meta          `path:"/admin/compliance/tax-invoices/{taxInvoiceNo}/submissions" method:"post" tags:"IDC Compliance Tax Invoice" summary:"Queue a tax invoice request" dc:"Recheck the current provider capability and queue one approved request as a separate action. This contract does not call a provider; a closed gate preserves waiting-for-submission and returns its blocker." permission:"idc:admin:compliance:tax-invoice:submit"`
	TaxInvoiceNo    string `json:"taxInvoiceNo" v:"required|max-length:64" dc:"Stable tax invoice request number" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed request version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"tax-invoice-submission-0001"`
}

// TaxInvoiceSubmissionCreateRes wraps the queued or safely blocked request.
type TaxInvoiceSubmissionCreateRes struct {
	Result *TaxInvoiceManagementActionResult `json:"result" dc:"Submission-queue result and current capability" eg:"{}"`
}
