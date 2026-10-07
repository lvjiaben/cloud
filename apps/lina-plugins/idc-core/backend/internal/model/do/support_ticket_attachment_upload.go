// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAttachmentUpload is the golang structure of table plugin_idc_core_support_ticket_attachment_upload for DAO operations like Where/Data.
type SupportTicketAttachmentUpload struct {
	g.Meta             `orm:"table:plugin_idc_core_support_ticket_attachment_upload, do:true"`
	Id                 any        //
	TenantId           any        //
	UploadNo           any        //
	TicketNo           any        //
	UploaderUserId     any        //
	UploaderType       any        //
	ContextCode        any        //
	FileName           any        //
	DeclaredSizeBytes  any        //
	MediaType          any        //
	Sha256Hex          any        //
	TemporaryObjectRef any        //
	StatusCode         any        //
	ExpiresAt          *time.Time //
	CompletedAt        *time.Time //
	ConsumedAt         *time.Time //
	Version            any        //
	CreatedAt          *time.Time //
	UpdatedAt          *time.Time //
}
