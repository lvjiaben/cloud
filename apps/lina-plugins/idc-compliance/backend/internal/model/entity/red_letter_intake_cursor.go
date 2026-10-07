// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RedLetterIntakeCursor is the golang structure for table red_letter_intake_cursor.
type RedLetterIntakeCursor struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	FeedCode     string     `json:"feedCode"     orm:"feed_code"     description:""`
	SourceCursor string     `json:"sourceCursor" orm:"source_cursor" description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
