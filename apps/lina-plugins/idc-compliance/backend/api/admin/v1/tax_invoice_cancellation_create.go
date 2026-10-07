// This file defines the tax-invoice cancellation action resource.
package v1

import "github.com/gogf/gf/v2/frame/g"

// TaxInvoiceCancellationCreateReq safely cancels one known-unissued request.
type TaxInvoiceCancellationCreateReq struct {
	g.Meta          `path:"/admin/compliance/tax-invoices/{taxInvoiceNo}/cancellations" method:"post" tags:"IDC Compliance Tax Invoice" summary:"Cancel a tax invoice request" dc:"Cancel only a pending-review, waiting-for-submission, or provider-failed request with positive not-issued evidence. The action uses a frozen customer-visible reason, optional administrator-only note, exact replay, and atomic audit before reserved capacity is released." permission:"idc:admin:compliance:tax-invoice:cancel"`
	TaxInvoiceNo    string `json:"taxInvoiceNo" v:"required|max-length:64" dc:"Stable tax invoice request number" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	ReasonCode      string `json:"reasonCode" v:"required|max-length:64" dc:"Closing reason: buyer_or_tax_information_incorrect, amount_or_billing_invoice_adjustment_required, duplicate_request, business_not_invoiceable, customer_requested_cancellation, order_or_refund_changed, or other" eg:"customer_requested_cancellation"`
	InternalNote    string `json:"internalNote" v:"max-length:2000" dc:"Optional administrator-only note; customer responses never expose it" eg:"The customer asked staff to cancel this request."`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed request version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"tax-invoice-cancellation-0001"`
}

// TaxInvoiceCancellationCreateRes wraps the canceled request.
type TaxInvoiceCancellationCreateRes struct {
	Result *TaxInvoiceManagementActionResult `json:"result" dc:"Canceled request result" eg:"{}"`
}
