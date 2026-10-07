// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EnterpriseMaterialUpload is the golang structure for table enterprise_material_upload.
type EnterpriseMaterialUpload struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	MaterialNo           string     `json:"materialNo"           orm:"material_no"            description:""`
	CaseNo               string     `json:"caseNo"               orm:"case_no"                description:""`
	CaseSubjectTypeCode  string     `json:"caseSubjectTypeCode"  orm:"case_subject_type_code" description:""`
	MaterialTypeCode     string     `json:"materialTypeCode"     orm:"material_type_code"     description:""`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:""`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
