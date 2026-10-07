// This file defines the tax-invoice request detail API.
package v1

import "github.com/gogf/gf/v2/frame/g"

// TaxInvoiceGetReq reads one safe staff projection.
type TaxInvoiceGetReq struct {
	g.Meta       `path:"/admin/compliance/tax-invoices/{taxInvoiceNo}" method:"get" tags:"IDC Compliance Tax Invoice" summary:"Get a tax invoice request" dc:"Return one bounded staff projection, current provider capability, and the immutable buyer profile selected at submission. Private snapshot references, bank-account secrets, and seller secrets are never returned. The administrator-only decision note is never part of customer APIs." permission:"idc:admin:compliance:tax-invoice:query"`
	TaxInvoiceNo string `json:"taxInvoiceNo" v:"required|max-length:64" dc:"Stable tax invoice request number" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
}

// TaxInvoiceGetRes wraps one safe request projection.
type TaxInvoiceGetRes struct {
	Item             *TaxInvoiceManagementItem  `json:"item" dc:"Safe staff tax invoice request projection" eg:"{}"`
	Buyer            TaxInvoiceBuyerProfileItem `json:"buyer" dc:"Immutable buyer profile selected when this request was submitted" eg:"{}"`
	Allocations      []TaxInvoiceAllocationItem `json:"allocations" dc:"Bounded immutable receivable allocations" eg:"[]"`
	Items            []TaxInvoiceLineItem       `json:"items" dc:"Bounded server-calculated tax lines" eg:"[]"`
	Files            []TaxInvoiceFileItem       `json:"files" dc:"Bounded immutable file metadata without private locations" eg:"[]"`
	HistoryTruncated bool                       `json:"historyTruncated" dc:"Whether any bounded evidence collection has more rows" eg:"false"`
}
