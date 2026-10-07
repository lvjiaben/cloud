// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAttachment is the golang structure of table plugin_idc_core_support_ticket_attachment for DAO operations like Where/Data.
type SupportTicketAttachment struct {
	g.Meta         `orm:"table:plugin_idc_core_support_ticket_attachment, do:true"`
	Id             any        //
	TenantId       any        //
	AttachmentNo   any        //
	TicketNo       any        //
	MessageNo      any        //
	VisibilityCode any        //
	FileName       any        //
	MediaType      any        //
	SizeBytes      any        //
	Sha256Hex      any        //
	ObjectRef      any        //
	InspectionCode any        //
	HiddenAt       *time.Time //
	HiddenBy       any        //
	HiddenReason   any        //
	CreatedBy      any        //
	CreatedAt      *time.Time //
}
