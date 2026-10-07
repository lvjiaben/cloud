// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketAttachment is the golang structure for table support_ticket_attachment.
type SupportTicketAttachment struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int64      `json:"tenantId"       orm:"tenant_id"       description:""`
	AttachmentNo   string     `json:"attachmentNo"   orm:"attachment_no"   description:""`
	TicketNo       string     `json:"ticketNo"       orm:"ticket_no"       description:""`
	MessageNo      string     `json:"messageNo"      orm:"message_no"      description:""`
	VisibilityCode string     `json:"visibilityCode" orm:"visibility_code" description:""`
	FileName       string     `json:"fileName"       orm:"file_name"       description:""`
	MediaType      string     `json:"mediaType"      orm:"media_type"      description:""`
	SizeBytes      int64      `json:"sizeBytes"      orm:"size_bytes"      description:""`
	Sha256Hex      string     `json:"sha256Hex"      orm:"sha256_hex"      description:""`
	ObjectRef      string     `json:"objectRef"      orm:"object_ref"      description:""`
	InspectionCode string     `json:"inspectionCode" orm:"inspection_code" description:""`
	HiddenAt       *time.Time `json:"hiddenAt"       orm:"hidden_at"       description:""`
	HiddenBy       int64      `json:"hiddenBy"       orm:"hidden_by"       description:""`
	HiddenReason   string     `json:"hiddenReason"   orm:"hidden_reason"   description:""`
	CreatedBy      int64      `json:"createdBy"      orm:"created_by"      description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
