// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerAttachment is the golang structure for table customer_attachment.
type CustomerAttachment struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"         description:""`
	AttachmentNo     string     `json:"attachmentNo"     orm:"attachment_no"     description:""`
	CustomerNo       string     `json:"customerNo"       orm:"customer_no"       description:""`
	CategoryCode     string     `json:"categoryCode"     orm:"category_code"     description:""`
	VisibilityCode   string     `json:"visibilityCode"   orm:"visibility_code"   description:""`
	ObjectKey        string     `json:"objectKey"        orm:"object_key"        description:"Internal private object key, never a public or long-lived download URL"`
	OriginalFilename string     `json:"originalFilename" orm:"original_filename" description:""`
	MediaType        string     `json:"mediaType"        orm:"media_type"        description:""`
	SizeBytes        int64      `json:"sizeBytes"        orm:"size_bytes"        description:""`
	ContentSha256    string     `json:"contentSha256"    orm:"content_sha256"    description:""`
	ScanStatusCode   string     `json:"scanStatusCode"   orm:"scan_status_code"  description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"       description:""`
	RetentionUntil   *time.Time `json:"retentionUntil"   orm:"retention_until"   description:""`
	UploadedBy       int        `json:"uploadedBy"       orm:"uploaded_by"       description:""`
	AvailableAt      *time.Time `json:"availableAt"      orm:"available_at"      description:""`
	RemovedAt        *time.Time `json:"removedAt"        orm:"removed_at"        description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"        description:""`
}
