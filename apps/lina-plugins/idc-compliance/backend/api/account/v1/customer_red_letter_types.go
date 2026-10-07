// This file defines customer-safe red-letter response projections.

package v1

// CustomerRedLetterTaskItem is the customer-safe correction projection.
type CustomerRedLetterTaskItem struct {
	TaskNo                       string `json:"taskNo" dc:"Stable red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
	TaxInvoiceNo                 string `json:"taxInvoiceNo" dc:"Related original tax invoice number when a single invoice applies" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	StatusCode                   string `json:"statusCode" dc:"Customer-safe correction status" eg:"awaiting_confirmation"`
	CustomerConfirmationRequired bool   `json:"customerConfirmationRequired" dc:"Whether the customer must complete supplier confirmation" eg:"true"`
	GrossAmountMinor             int64  `json:"grossAmountMinor" dc:"Gross correction amount in the smallest currency unit" eg:"10000"`
	CurrencyCode                 string `json:"currencyCode" dc:"Three-letter uppercase currency" eg:"CNY"`
	Version                      int64  `json:"version" dc:"Optimistic task version" eg:"1"`
	CreatedAt                    int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1767225600000"`
	CompletedAt                  int64  `json:"completedAt" dc:"Completion time as Unix milliseconds, or zero before completion" eg:"0"`
	CustomerActionURL            string `json:"customerActionUrl" dc:"Short-lived supplier confirmation URL when currently available" eg:"https://example.invalid/confirm"`
}
