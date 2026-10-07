// This file defines the authenticated customer's bounded tax-invoice request list.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerTaxInvoiceListReq lists only the authenticated customer's requests.
type CustomerTaxInvoiceListReq struct {
	g.Meta `path:"/account/compliance/tax-invoices" method:"get" tags:"IDC Compliance Account" summary:"List my tax-invoice requests" dc:"Return one stable bounded page owned by the authenticated customer without an exact total count or supplier I/O."`
	Cursor string `json:"cursor" v:"max-length:256" dc:"Opaque cursor returned by the previous page" eg:"eyJzdWJtaXR0ZWRBdCI6IjIwMjYtMDgtMTdUMTA6MDA6MDBaIiwiaWQiOjF9"`
	Limit  int    `json:"limit" v:"min:1|max:50" d:"20" dc:"Bounded page size from one to fifty" eg:"20"`
}

// CustomerTaxInvoiceListRes returns one bounded customer-owned page.
type CustomerTaxInvoiceListRes struct {
	Items      []CustomerTaxInvoiceItem `json:"items" dc:"Bounded customer-owned request page" eg:"[]"`
	NextCursor string                   `json:"nextCursor" dc:"Opaque next-page cursor; empty when finished" eg:""`
}
