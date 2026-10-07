// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerNote is the golang structure for table customer_note.
type CustomerNote struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	NoteNo           string     `json:"noteNo"           orm:"note_no"            description:""`
	CustomerNo       string     `json:"customerNo"       orm:"customer_no"        description:""`
	CategoryCode     string     `json:"categoryCode"     orm:"category_code"      description:""`
	Content          string     `json:"content"          orm:"content"            description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"        description:""`
	SupersedesNoteNo string     `json:"supersedesNoteNo" orm:"supersedes_note_no" description:""`
	AuthorUserId     int        `json:"authorUserId"     orm:"author_user_id"     description:""`
	WithdrawnBy      int        `json:"withdrawnBy"      orm:"withdrawn_by"       description:""`
	WithdrawnAt      *time.Time `json:"withdrawnAt"      orm:"withdrawn_at"       description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"         description:""`
}
