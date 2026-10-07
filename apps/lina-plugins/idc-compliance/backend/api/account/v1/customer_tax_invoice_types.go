// This file defines customer-safe tax-invoice request projections.

package v1

// CustomerTaxInvoiceProfile is one current customer's saved buyer title.
type CustomerTaxInvoiceProfile struct {
	ProfileNo         string `json:"profileNo" dc:"Opaque saved buyer-title number" eg:"profile_0123456789abcdef"`
	TitleTypeCode     string `json:"titleTypeCode" dc:"Saved title type" eg:"enterprise"`
	InvoiceTitle      string `json:"invoiceTitle" dc:"Saved buyer invoice title" eg:"Example Company"`
	TaxRegistrationNo string `json:"taxRegistrationNo" dc:"Saved buyer tax registration number" eg:"91110000123456789X"`
	RegisteredAddress string `json:"registeredAddress" dc:"Saved registered address" eg:"Beijing"`
	RegisteredPhone   string `json:"registeredPhone" dc:"Saved registered phone" eg:"010-12345678"`
	BankName          string `json:"bankName" dc:"Saved bank name" eg:"Example Bank"`
	BankAccountMasked string `json:"bankAccountMasked" dc:"Masked saved bank account" eg:"****6789"`
	IsDefault         bool   `json:"isDefault" dc:"Whether this is the current default title" eg:"true"`
	Version           int64  `json:"version" dc:"Current saved-title version" eg:"2"`
}

// CustomerTaxInvoiceSource is one paid billing invoice with available capacity.
type CustomerTaxInvoiceSource struct {
	BillingInvoiceNo      string `json:"billingInvoiceNo" dc:"Current customer billing invoice number" eg:"INV_01J2ABCDEF"`
	OrderNo               string `json:"orderNo" dc:"Related current customer order number" eg:"ORD_01J2ABCDEF"`
	CurrencyCode          string `json:"currencyCode" dc:"ISO 4217 billing currency" eg:"CNY"`
	SourceGrossMinor      string `json:"sourceGrossMinor" dc:"Paid billing total in decimal minor units" eg:"10000"`
	AlreadyAllocatedMinor string `json:"alreadyAllocatedMinor" dc:"Amount already reserved or issued in decimal minor units" eg:"2000"`
	RemainingGrossMinor   string `json:"remainingGrossMinor" dc:"Current available amount in decimal minor units" eg:"8000"`
	SettledAt             int64  `json:"settledAt" dc:"Authoritative settlement time as Unix milliseconds" eg:"1786502400000"`
}

// CustomerTaxInvoiceItem is one customer-safe tax-invoice request summary.
type CustomerTaxInvoiceItem struct {
	TaxInvoiceNo       string `json:"taxInvoiceNo" dc:"Opaque customer-owned tax-invoice request number" eg:"TINV_0123456789ABCDEF"`
	StatusCode         string `json:"statusCode" dc:"Current supplier-neutral request status" eg:"pending_review"`
	InvoiceTypeCode    string `json:"invoiceTypeCode" dc:"Confirmed invoice type or pending_confirmation" eg:"pending_confirmation"`
	CurrencyCode       string `json:"currencyCode" dc:"ISO 4217 tax currency" eg:"CNY"`
	GrossAmountMinor   string `json:"grossAmountMinor" dc:"Requested gross total in decimal minor units" eg:"10000"`
	MaskedInvoiceNo    string `json:"maskedInvoiceNo,omitempty" dc:"Masked issued invoice number when available" eg:"****1234"`
	DecisionReasonCode string `json:"decisionReasonCode,omitempty" dc:"Stable customer-visible rejection or cancellation reason" eg:"amount_or_billing_invoice_adjustment_required"`
	SubmittedAt        int64  `json:"submittedAt" dc:"Submission time as Unix milliseconds" eg:"1786502400000"`
	IssuedAt           int64  `json:"issuedAt,omitempty" dc:"Authoritative issue time as Unix milliseconds" eg:"1786502500000"`
	DeliveredAt        int64  `json:"deliveredAt,omitempty" dc:"Authoritative delivery time as Unix milliseconds" eg:"1786502600000"`
	Version            int64  `json:"version" dc:"Current request version" eg:"2"`
}
