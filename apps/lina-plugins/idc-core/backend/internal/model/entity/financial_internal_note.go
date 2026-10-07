// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialInternalNote is the golang structure for table financial_internal_note.
type FinancialInternalNote struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	NoteNo            string     `json:"noteNo"            orm:"note_no"            description:""`
	TargetTypeCode    string     `json:"targetTypeCode"    orm:"target_type_code"   description:""`
	TargetNo          string     `json:"targetNo"          orm:"target_no"          description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"        description:""`
	Content           string     `json:"content"           orm:"content"            description:""`
	CorrectsNoteNo    string     `json:"correctsNoteNo"    orm:"corrects_note_no"   description:""`
	CreatedBy         int        `json:"createdBy"         orm:"created_by"         description:""`
	IdempotencyDigest string     `json:"idempotencyDigest" orm:"idempotency_digest" description:""`
	RequestDigest     string     `json:"requestDigest"     orm:"request_digest"     description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
}
