// This file defines safe tax-invoice management response types.
package v1

// TaxInvoiceManagementItem is one staff-only safe request projection.
type TaxInvoiceManagementItem struct {
	TaxInvoiceNo          string   `json:"taxInvoiceNo" dc:"Stable tax invoice request number" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	CustomerNo            string   `json:"customerNo" dc:"Customer number that owns this request" eg:"CUS_0123456789ABCDEF"`
	StatusCode            string   `json:"statusCode" dc:"Request status: draft, pending_review, approved_pending_submission, rejected, queued, submitting, issued, delivered, failed, result_unknown, red_letter_pending, red_lettered, or canceled" eg:"approved_pending_submission"`
	InvoiceTypeCode       string   `json:"invoiceTypeCode" dc:"Confirmed type or pending_confirmation: vat_electronic_normal, vat_electronic_special, digital_electronic_normal, or digital_electronic_special" eg:"vat_electronic_normal"`
	ProviderCode          string   `json:"providerCode" dc:"Installation-local invoice provider code; empty before approval" eg:"nuonuo_invoice"`
	DecisionReasonCode    string   `json:"decisionReasonCode" dc:"Stable customer-visible rejection or cancellation reason; empty while open" eg:"duplicate_request"`
	DecisionInternalNote  string   `json:"decisionInternalNote" dc:"Optional administrator-only note that customer APIs must never expose" eg:"The customer confirmed this was a duplicate request."`
	GrossAmountMinor      int64    `json:"grossAmountMinor" dc:"Gross invoice amount in the currency smallest unit" eg:"128800"`
	NetAmountMinor        int64    `json:"netAmountMinor" dc:"Net amount in the currency smallest unit" eg:"121509"`
	TaxAmountMinor        int64    `json:"taxAmountMinor" dc:"Tax amount in the currency smallest unit" eg:"7291"`
	CurrencyCode          string   `json:"currencyCode" dc:"Uppercase ISO 4217 tax currency code" eg:"CNY"`
	MaskedInvoiceNumber   string   `json:"maskedInvoiceNumber" dc:"Masked issued invoice number; empty before issuance" eg:"****8890"`
	Version               int64    `json:"version" dc:"Optimistic concurrency version" eg:"2"`
	SubmittedAt           int64    `json:"submittedAt" dc:"Submission Unix timestamp in milliseconds" eg:"1786464060000"`
	ReviewedAt            *int64   `json:"reviewedAt" dc:"Review Unix timestamp in milliseconds; null before review" eg:"1786464120000"`
	IssuedAt              int64    `json:"issuedAt" dc:"Issue Unix timestamp in milliseconds; zero before issuance" eg:"1786464180000"`
	DeliveredAt           int64    `json:"deliveredAt" dc:"Delivery Unix timestamp in milliseconds; zero before delivery" eg:"1786464240000"`
	CanceledAt            int64    `json:"canceledAt" dc:"Cancellation Unix timestamp in milliseconds; zero while open" eg:"0"`
	ProviderNotIssuedAt   int64    `json:"providerNotIssuedAt" dc:"Authoritative not-issued evidence time in Unix milliseconds; zero when absent" eg:"0"`
	CapabilityReady       bool     `json:"capabilityReady" dc:"Whether the current installation provider gate permits approval and submission" eg:"false"`
	CapabilityReasonCode  string   `json:"capabilityReasonCode" dc:"Stable blocker when capabilityReady is false; empty when ready" eg:"external_confirmation_required"`
	SupportedInvoiceTypes []string `json:"supportedInvoiceTypes" dc:"Bounded provider-confirmed types selectable by staff; empty while the gate is closed" eg:"[\"vat_electronic_normal\"]"`
}

type TaxInvoiceAllocationItem struct {
	AllocationNo          string `json:"allocationNo" dc:"Immutable local allocation number" eg:"TIAL_0123456789ABCDEF"`
	BillingInvoiceNo      string `json:"billingInvoiceNo" dc:"Opaque receivable number" eg:"BILL_0123456789ABCDEF"`
	OrderNo               string `json:"orderNo" dc:"Opaque order number" eg:"ORDER_0123456789ABCDEF"`
	GrossAmountMinor      int64  `json:"grossAmountMinor" dc:"Allocated gross amount" eg:"128800"`
	NetAmountMinor        int64  `json:"netAmountMinor" dc:"Allocated net amount" eg:"121509"`
	TaxAmountMinor        int64  `json:"taxAmountMinor" dc:"Allocated tax amount" eg:"7291"`
	RefundedGrossMinor    int64  `json:"refundedGrossMinor" dc:"Refunded amount tracked for this allocation" eg:"0"`
	RedLetteredGrossMinor int64  `json:"redLetteredGrossMinor" dc:"Completed red-letter amount" eg:"0"`
}

// TaxInvoiceBuyerProfileItem is the immutable buyer profile selected by the
// customer. It intentionally omits every private snapshot and bank-secret reference.
type TaxInvoiceBuyerProfileItem struct {
	ProfileNo         string `json:"profileNo" dc:"Frozen customer invoice-profile number" eg:"IPRF_0123456789ABCDEF"`
	TitleTypeCode     string `json:"titleTypeCode" dc:"Frozen buyer title type: personal or enterprise" eg:"enterprise"`
	InvoiceTitle      string `json:"invoiceTitle" dc:"Frozen buyer invoice title reviewed by staff" eg:"Example Buyer Co., Ltd."`
	TaxRegistrationNo string `json:"taxRegistrationNo" dc:"Frozen buyer tax registration number; empty for a personal title" eg:"91310000123456789X"`
	RegisteredAddress string `json:"registeredAddress" dc:"Frozen optional buyer registered address" eg:"Shanghai"`
	RegisteredPhone   string `json:"registeredPhone" dc:"Frozen optional buyer registered phone" eg:"021-12345678"`
	BankName          string `json:"bankName" dc:"Frozen optional buyer bank name" eg:"Example Bank"`
	BankAccountMasked string `json:"bankAccountMasked" dc:"Frozen masked buyer bank account; plaintext is never returned" eg:"****8890"`
	Version           int64  `json:"version" dc:"Frozen invoice-profile version" eg:"2"`
}

type TaxInvoiceLineItem struct {
	ItemNo                string `json:"itemNo" dc:"Immutable tax line number" eg:"TIIT_0123456789ABCDEF"`
	AllocationNo          string `json:"allocationNo" dc:"Owning allocation number" eg:"TIAL_0123456789ABCDEF"`
	BillingInvoiceItemNo  string `json:"billingInvoiceItemNo" dc:"Opaque billing line number" eg:"BIIT_0123456789ABCDEF"`
	LineNo                int    `json:"lineNo" dc:"Stable display order" eg:"1"`
	TaxClassificationCode string `json:"taxClassificationCode" dc:"Frozen tax classification" eg:"304020101"`
	TaxRatePPM            int    `json:"taxRatePpm" dc:"Frozen tax rate in parts per million" eg:"60000"`
	GrossAmountMinor      int64  `json:"grossAmountMinor" dc:"Line gross amount" eg:"128800"`
	NetAmountMinor        int64  `json:"netAmountMinor" dc:"Line net amount" eg:"121509"`
	TaxAmountMinor        int64  `json:"taxAmountMinor" dc:"Line tax amount" eg:"7291"`
}

type TaxInvoiceFileItem struct {
	FileNo        string `json:"fileNo" dc:"Immutable file metadata number" eg:"TIFL_0123456789ABCDEF"`
	FileTypeCode  string `json:"fileTypeCode" dc:"Invoice file type" eg:"invoice_pdf"`
	ContentSHA256 string `json:"contentSha256" dc:"File SHA-256 integrity digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	MediaType     string `json:"mediaType" dc:"Stored media type" eg:"application/pdf"`
	ByteSize      int64  `json:"byteSize" dc:"Stored file size" eg:"102400"`
	StatusCode    string `json:"statusCode" dc:"File archive state" eg:"archived"`
	Immutable     bool   `json:"immutable" dc:"Whether content metadata is immutable" eg:"true"`
	ArchivedAt    int64  `json:"archivedAt" dc:"Archive Unix timestamp in milliseconds; zero before archive" eg:"1786464240000"`
	CreatedAt     int64  `json:"createdAt" dc:"Creation Unix timestamp in milliseconds" eg:"1786464180000"`
}

// TaxInvoiceManagementActionResult reports an applied, replayed, or safely blocked action.
type TaxInvoiceManagementActionResult struct {
	Item    *TaxInvoiceManagementItem `json:"item" dc:"Current safe request projection" eg:"{}"`
	Applied bool                      `json:"applied" dc:"Whether this action reached its intended local state; false when the provider gate safely blocked it" eg:"false"`
	Reused  bool                      `json:"reused" dc:"Whether an exact earlier action result was replayed" eg:"false"`
}
