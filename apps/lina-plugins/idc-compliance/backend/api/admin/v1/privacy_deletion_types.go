// Privacy deletion API types describe safe task state and confirmation evidence. They never contain the private material being removed.

package v1

type PrivacyDeletionItem struct {
	TaskNo            string `json:"taskNo" dc:"Opaque deletion task number" eg:"PDT_0123456789abcdef0123456789abcdef"`
	DataTypeCode      string `json:"dataTypeCode" dc:"Governed data type: personal_verification_material, enterprise_verification_material, or tax_invoice_evidence" eg:"personal_verification_material"`
	TargetTypeCode    string `json:"targetTypeCode" dc:"Bounded local record type selected for deletion" eg:"personal_verification_case"`
	TargetNo          string `json:"targetNo" dc:"Opaque number of the local record selected for deletion" eg:"PVC_0123456789abcdef0123456789abcdef"`
	StatusCode        string `json:"statusCode" dc:"Task state: pending, blocked, confirmed with unfinished disposition, or completed" eg:"pending"`
	DueAt             int64  `json:"dueAt" dc:"Deletion due time as a Unix timestamp in milliseconds" eg:"1787040000000"`
	HoldReasonCode    string `json:"holdReasonCode" dc:"Blocking reason: tax, dispute, audit, legal_hold, or empty" eg:"legal_hold"`
	ConfirmedByUserID int    `json:"confirmedByUserId" dc:"Confirming administrator identifier; zero before confirmation" eg:"1"`
	ConfirmedAt       int64  `json:"confirmedAt" dc:"Confirmation time as a Unix timestamp in milliseconds; zero before confirmation" eg:"1787040000000"`
	CompletedAt       int64  `json:"completedAt" dc:"Completion time as a Unix timestamp in milliseconds; zero before completion" eg:"1787040000000"`
	Version           int64  `json:"version" dc:"Current optimistic concurrency version" eg:"1"`
}
