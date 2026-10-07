// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentFile is the golang structure for table content_file.
type ContentFile struct {
	Id                  int64      `json:"id"                  orm:"id"                   description:""`
	TenantId            int64      `json:"tenantId"            orm:"tenant_id"            description:""`
	FileNo              string     `json:"fileNo"              orm:"file_no"              description:""`
	ContentNo           string     `json:"contentNo"           orm:"content_no"           description:""`
	UploadNo            string     `json:"uploadNo"            orm:"upload_no"            description:""`
	CandidateGeneration int64      `json:"candidateGeneration" orm:"candidate_generation" description:""`
	RoleCode            string     `json:"roleCode"            orm:"role_code"            description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"          description:""`
	FileName            string     `json:"fileName"            orm:"file_name"            description:""`
	MediaType           string     `json:"mediaType"           orm:"media_type"           description:""`
	ContentKind         string     `json:"contentKind"         orm:"content_kind"         description:""`
	SizeBytes           int64      `json:"sizeBytes"           orm:"size_bytes"           description:""`
	Sha256Hex           string     `json:"sha256Hex"           orm:"sha256_hex"           description:""`
	ObjectRef           string     `json:"objectRef"           orm:"object_ref"           description:""`
	ScanRequired        bool       `json:"scanRequired"        orm:"scan_required"        description:""`
	ScanStatus          string     `json:"scanStatus"          orm:"scan_status"          description:""`
	GateCompletedAt     *time.Time `json:"gateCompletedAt"     orm:"gate_completed_at"    description:""`
	RetentionExpiresAt  *time.Time `json:"retentionExpiresAt"  orm:"retention_expires_at" description:""`
	CancelledAt         *time.Time `json:"cancelledAt"         orm:"cancelled_at"         description:""`
	EnabledAt           *time.Time `json:"enabledAt"           orm:"enabled_at"           description:""`
	ObjectDeletedAt     *time.Time `json:"objectDeletedAt"     orm:"object_deleted_at"    description:""`
	Version             int64      `json:"version"             orm:"version"              description:""`
	CreatedBy           int64      `json:"createdBy"           orm:"created_by"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"           description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"           description:""`
}
