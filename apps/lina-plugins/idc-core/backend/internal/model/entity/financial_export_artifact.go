// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialExportArtifact is the golang structure for table financial_export_artifact.
type FinancialExportArtifact struct {
	Id            int64      `json:"id"            orm:"id"             description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	RequestNo     string     `json:"requestNo"     orm:"request_no"     description:""`
	ObjectRef     string     `json:"objectRef"     orm:"object_ref"     description:""`
	ContentSha256 string     `json:"contentSha256" orm:"content_sha256" description:""`
	SizeBytes     int64      `json:"sizeBytes"     orm:"size_bytes"     description:""`
	RowCount      int        `json:"rowCount"      orm:"row_count"      description:""`
	StatusCode    string     `json:"statusCode"    orm:"status_code"    description:""`
	ExpiresAt     *time.Time `json:"expiresAt"     orm:"expires_at"     description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
	UpdatedAt     *time.Time `json:"updatedAt"     orm:"updated_at"     description:""`
}
