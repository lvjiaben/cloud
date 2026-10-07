// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyAiField is the golang structure for table privacy_ai_field.
type PrivacyAiField struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	AssessmentNo string     `json:"assessmentNo" orm:"assessment_no" description:""`
	FieldCode    string     `json:"fieldCode"    orm:"field_code"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
