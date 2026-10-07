// This file defines bounded staff red-letter request and response projections.

package v1

// RedLetterTaskItem is a bounded staff projection without private evidence locations.
type RedLetterTaskItem struct {
	TaskNo                       string `json:"taskNo" dc:"Stable red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
	RefundOperationNo            string `json:"refundOperationNo" dc:"Opaque completed refund operation number" eg:"refund_0123456789abcdef"`
	TaxInvoiceNo                 string `json:"taxInvoiceNo" dc:"Single original tax invoice number when all allocations resolve to one invoice, otherwise empty" eg:"TINV_0123456789ABCDEF0123456789ABCDEF"`
	CustomerNo                   string `json:"customerNo" dc:"Opaque customer number" eg:"CUST_0123456789ABCDEF0123456789ABCDEF"`
	CurrencyCode                 string `json:"currencyCode" dc:"Three-letter uppercase tax currency" eg:"CNY"`
	StatusCode                   string `json:"statusCode" dc:"Stable red-letter task status" eg:"manual_required"`
	ManualReasonCode             string `json:"manualReasonCode" dc:"Stable reason when staff action is required" eg:"provider_automation_unavailable"`
	AllocationResolved           bool   `json:"allocationResolved" dc:"Whether every refund amount is linked to exact original invoice allocations" eg:"true"`
	CustomerConfirmationRequired bool   `json:"customerConfirmationRequired" dc:"Whether the supplier requires buyer confirmation" eg:"false"`
	GrossAmountMinor             int64  `json:"grossAmountMinor" dc:"Gross correction amount in the smallest currency unit" eg:"10000"`
	NetAmountMinor               int64  `json:"netAmountMinor" dc:"Net correction amount after exact allocation" eg:"9434"`
	TaxAmountMinor               int64  `json:"taxAmountMinor" dc:"Tax correction amount after exact allocation" eg:"566"`
	Version                      int64  `json:"version" dc:"Optimistic task version" eg:"1"`
	CreatedAt                    int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1767225600000"`
	CompletedAt                  int64  `json:"completedAt" dc:"Completion time as Unix milliseconds, or zero before completion" eg:"0"`
	CandidateCount               int    `json:"candidateCount" dc:"Number of bounded eligible original-invoice choices" eg:"1"`
	AllocationCount              int    `json:"allocationCount" dc:"Number of immutable final correction allocations" eg:"1"`
	CompletionEvidenceNo         string `json:"completionEvidenceNo" dc:"Immutable completion evidence number; empty before completion" eg:"rle_0123456789abcdef0123456789abcdef01234567"`
}

type RedLetterCandidateItem struct {
	RefundAllocationNo     string `json:"refundAllocationNo" dc:"Opaque refund allocation number" eg:"refa_0123456789abcdef"`
	TaxInvoiceNo           string `json:"taxInvoiceNo" dc:"Eligible original tax invoice" eg:"TINV_0123456789ABCDEF"`
	TaxInvoiceAllocationNo string `json:"taxInvoiceAllocationNo" dc:"Eligible original invoice allocation" eg:"TIAL_0123456789ABCDEF"`
	TaxInvoiceItemNo       string `json:"taxInvoiceItemNo" dc:"Eligible original tax line" eg:"TIIT_0123456789ABCDEF"`
	RequestedGrossMinor    int64  `json:"requestedGrossMinor" dc:"Refund amount requiring allocation" eg:"10000"`
	AvailableGrossMinor    int64  `json:"availableGrossMinor" dc:"Maximum available gross correction" eg:"10000"`
	AvailableNetMinor      int64  `json:"availableNetMinor" dc:"Corresponding net amount" eg:"9434"`
	AvailableTaxMinor      int64  `json:"availableTaxMinor" dc:"Corresponding tax amount" eg:"566"`
}

type RedLetterAllocationItem struct {
	RefundAllocationNo     string `json:"refundAllocationNo" dc:"Opaque refund allocation number" eg:"refa_0123456789abcdef"`
	TaxInvoiceNo           string `json:"taxInvoiceNo" dc:"Resolved original tax invoice" eg:"TINV_0123456789ABCDEF"`
	TaxInvoiceAllocationNo string `json:"taxInvoiceAllocationNo" dc:"Resolved original invoice allocation" eg:"TIAL_0123456789ABCDEF"`
	TaxInvoiceItemNo       string `json:"taxInvoiceItemNo" dc:"Resolved original tax line" eg:"TIIT_0123456789ABCDEF"`
	GrossAmountMinor       int64  `json:"grossAmountMinor" dc:"Correction gross amount" eg:"10000"`
	NetAmountMinor         int64  `json:"netAmountMinor" dc:"Correction net amount" eg:"9434"`
	TaxAmountMinor         int64  `json:"taxAmountMinor" dc:"Correction tax amount" eg:"566"`
}

type RedLetterEvidenceItem struct {
	EvidenceNo        string `json:"evidenceNo" dc:"Immutable evidence number" eg:"rle_0123456789abcdef0123456789abcdef01234567"`
	EvidenceReference string `json:"evidenceReference" dc:"Stable real-world evidence reference" eg:"red-confirmation-20260817-001"`
	EvidenceSHA256    string `json:"evidenceSha256" dc:"Evidence SHA-256 digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	MediaType         string `json:"mediaType" dc:"Optional uploaded evidence media type" eg:"application/pdf"`
	ByteSize          int64  `json:"byteSize" dc:"Optional uploaded evidence size" eg:"102400"`
	OccurredAt        int64  `json:"occurredAt" dc:"Evidence occurrence Unix timestamp in milliseconds" eg:"1767225600000"`
}

// RedLetterAllocationSelection selects one eligible original invoice allocation.
type RedLetterAllocationSelection struct {
	RefundAllocationNo     string `json:"refundAllocationNo" v:"required|max-length:64" dc:"Opaque core refund allocation number" eg:"refa_0123456789abcdef"`
	TaxInvoiceAllocationNo string `json:"taxInvoiceAllocationNo" v:"required|max-length:64" dc:"Eligible original tax-invoice allocation number" eg:"TIAL_0123456789ABCDEF0123456789ABCDEF"`
	GrossAmountMinor       int64  `json:"grossAmountMinor" v:"required|min:1" dc:"Gross amount assigned to this original invoice in the smallest currency unit" eg:"10000"`
}
