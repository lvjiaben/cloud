// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractFile is the golang structure for table contract_file.
type ContractFile struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	FileNo             string     `json:"fileNo"             orm:"file_no"              description:""`
	ContractNo         string     `json:"contractNo"         orm:"contract_no"          description:""`
	FlowNo             *string    `json:"flowNo"             orm:"flow_no"              description:""`
	FileTypeCode       string     `json:"fileTypeCode"       orm:"file_type_code"       description:""`
	PrivateObjectRef   string     `json:"privateObjectRef"   orm:"private_object_ref"   description:""`
	ContentSha256      string     `json:"contentSha256"      orm:"content_sha256"       description:""`
	MediaType          string     `json:"mediaType"          orm:"media_type"           description:""`
	ByteSize           int64      `json:"byteSize"           orm:"byte_size"            description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	Immutable          bool       `json:"immutable"          orm:"immutable"            description:""`
	ArchivedAt         *time.Time `json:"archivedAt"         orm:"archived_at"          description:""`
	RetentionUntil     *time.Time `json:"retentionUntil"     orm:"retention_until"      description:""`
	LegalHold          bool       `json:"legalHold"          orm:"legal_hold"           description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
	SupplierObservedAt *time.Time `json:"supplierObservedAt" orm:"supplier_observed_at" description:""`
}
