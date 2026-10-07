// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketAttachmentUpload is the golang structure for table support_ticket_attachment_upload.
type SupportTicketAttachmentUpload struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int64      `json:"tenantId"           orm:"tenant_id"            description:""`
	UploadNo           string     `json:"uploadNo"           orm:"upload_no"            description:""`
	TicketNo           string     `json:"ticketNo"           orm:"ticket_no"            description:""`
	UploaderUserId     int64      `json:"uploaderUserId"     orm:"uploader_user_id"     description:""`
	UploaderType       string     `json:"uploaderType"       orm:"uploader_type"        description:""`
	ContextCode        string     `json:"contextCode"        orm:"context_code"         description:""`
	FileName           string     `json:"fileName"           orm:"file_name"            description:""`
	DeclaredSizeBytes  int64      `json:"declaredSizeBytes"  orm:"declared_size_bytes"  description:""`
	MediaType          string     `json:"mediaType"          orm:"media_type"           description:""`
	Sha256Hex          string     `json:"sha256Hex"          orm:"sha256_hex"           description:""`
	TemporaryObjectRef string     `json:"temporaryObjectRef" orm:"temporary_object_ref" description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	ExpiresAt          *time.Time `json:"expiresAt"          orm:"expires_at"           description:""`
	CompletedAt        *time.Time `json:"completedAt"        orm:"completed_at"         description:""`
	ConsumedAt         *time.Time `json:"consumedAt"         orm:"consumed_at"          description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
}
