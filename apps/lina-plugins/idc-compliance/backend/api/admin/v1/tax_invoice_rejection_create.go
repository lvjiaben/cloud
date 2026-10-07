// This file defines the tax-invoice rejection action resource.
package v1

import "github.com/gogf/gf/v2/frame/g"

// TaxInvoiceRejectionCreateReq rejects one pending request.
type TaxInvoiceRejectionCreateReq struct {
	g.Meta          `path:"/admin/compliance/tax-invoices/{taxInvoiceNo}/rejections" method:"post" tags:"IDC Compliance Tax Invoice" summary:"Reject a tax invoice request" dc:"Reject one pending request with a frozen customer-visible reason, optional administrator-only note, optimistic concurrency, exact replay, and atomic audit. Rejection releases reserved invoice capacity." permission:"idc:admin:compliance:tax-invoice:review"`
	TaxInvoiceNo    string `json:"taxInvoiceNo" v:"required|max-length:64" dc:"Stable tax invoice request number" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	ReasonCode      string `json:"reasonCode" v:"required|max-length:64" dc:"Closing reason: buyer_or_tax_information_incorrect, amount_or_billing_invoice_adjustment_required, duplicate_request, business_not_invoiceable, customer_requested_cancellation, order_or_refund_changed, or other" eg:"duplicate_request"`
	InternalNote    string `json:"internalNote" v:"max-length:2000" dc:"Optional administrator-only note; customer responses never expose it" eg:"The customer confirmed this was a duplicate request."`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed request version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"tax-invoice-rejection-0001"`
}

// TaxInvoiceRejectionCreateRes wraps the rejected request.
type TaxInvoiceRejectionCreateRes struct {
	Result *TaxInvoiceManagementActionResult `json:"result" dc:"Rejected request result" eg:"{}"`
}
