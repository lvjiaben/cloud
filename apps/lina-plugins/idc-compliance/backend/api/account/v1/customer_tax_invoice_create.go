// This file defines customer submission of one supplier-neutral tax-invoice request.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerTaxInvoiceSelection selects one paid bill and requested gross amount.
type CustomerTaxInvoiceSelection struct {
	BillingInvoiceNo    string `json:"billingInvoiceNo" v:"required|max-length:64" dc:"Current customer paid billing invoice number" eg:"INV_01J2ABCDEF"`
	RequestedGrossMinor string `json:"requestedGrossMinor" v:"required|regex:^\\d{1,19}$" dc:"Positive requested gross amount in decimal minor units" eg:"8000"`
}

// CustomerTaxInvoiceCreateReq creates or safely recovers one immutable request.
type CustomerTaxInvoiceCreateReq struct {
	g.Meta         `path:"/account/compliance/tax-invoices" method:"post" tags:"IDC Compliance Account" summary:"Create my tax-invoice request" dc:"Create one immutable pending-review request. Tax, seller, customer, invoice type, and item facts are calculated exclusively by the server."`
	ProfileNo      string                        `json:"profileNo" v:"required|max-length:64" dc:"Selected current saved buyer-title number" eg:"profile_0123456789abcdef"`
	Selections     []CustomerTaxInvoiceSelection `json:"selections" v:"required" dc:"One to twenty current paid billing invoice selections; collection bounds are enforced by the invoice service" eg:"[]"`
	IdempotencyKey string                        `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"tax-invoice-01J2ABCDEF"`
}

// CustomerTaxInvoiceCreateRes returns the safe created or replayed request.
type CustomerTaxInvoiceCreateRes struct {
	Item   CustomerTaxInvoiceItem `json:"item" dc:"Created customer-safe request" eg:"{}"`
	Reused bool                   `json:"reused" dc:"Whether an exact earlier submission was recovered" eg:"false"`
}
