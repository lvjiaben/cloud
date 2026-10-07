// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacySupportExport is the golang structure for table privacy_support_export.
type PrivacySupportExport struct {
	Id              int64      `json:"id"              orm:"id"                 description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"          description:""`
	ExportNo        string     `json:"exportNo"        orm:"export_no"          description:""`
	PurposeCode     string     `json:"purposeCode"     orm:"purpose_code"       description:""`
	AssessmentNo    string     `json:"assessmentNo"    orm:"assessment_no"      description:""`
	FieldCount      int        `json:"fieldCount"      orm:"field_count"        description:""`
	RecordCount     int        `json:"recordCount"     orm:"record_count"       description:""`
	ContentSha256   string     `json:"contentSha256"   orm:"content_sha256"     description:""`
	ContentBytes    []byte     `json:"contentBytes"    orm:"content_bytes"      description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"        description:""`
	RetentionUntil  *time.Time `json:"retentionUntil"  orm:"retention_until"    description:""`
	CreatedByUserId int        `json:"createdByUserId" orm:"created_by_user_id" description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"         description:""`
}
