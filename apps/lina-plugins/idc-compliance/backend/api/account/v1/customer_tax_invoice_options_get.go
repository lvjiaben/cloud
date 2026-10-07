// This file defines the current customer's bounded tax-invoice request options.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerTaxInvoiceOptionsGetReq reads current saved titles and eligible paid bills.
type CustomerTaxInvoiceOptionsGetReq struct {
	g.Meta `path:"/account/compliance/tax-invoice-options" method:"get" tags:"IDC Compliance Account" summary:"Get my tax-invoice request options" dc:"Return bounded current-customer buyer titles and paid billing invoices after current reservations. No customer identifier is accepted."`
}

// CustomerTaxInvoiceOptionsGetRes returns one server-calculated picker snapshot.
type CustomerTaxInvoiceOptionsGetRes struct {
	Profiles   []CustomerTaxInvoiceProfile `json:"profiles" dc:"At most fifty current saved buyer titles" eg:"[]"`
	Invoices   []CustomerTaxInvoiceSource  `json:"invoices" dc:"At most fifty currently invoiceable paid billing invoices" eg:"[]"`
	ObservedAt int64                       `json:"observedAt" dc:"Server observation time as Unix milliseconds" eg:"1786502400000"`
}
