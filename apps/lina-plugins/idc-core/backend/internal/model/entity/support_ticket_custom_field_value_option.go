// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketCustomFieldValueOption is the golang structure for table support_ticket_custom_field_value_option.
type SupportTicketCustomFieldValueOption struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	TenantId  int64      `json:"tenantId"  orm:"tenant_id"  description:""`
	TicketNo  string     `json:"ticketNo"  orm:"ticket_no"  description:""`
	FieldNo   string     `json:"fieldNo"   orm:"field_no"   description:""`
	OptionNo  string     `json:"optionNo"  orm:"option_no"  description:""`
	NameZhCn  string     `json:"nameZhCn"  orm:"name_zh_cn" description:""`
	NameEnUs  string     `json:"nameEnUs"  orm:"name_en_us" description:""`
	SortOrder int        `json:"sortOrder" orm:"sort_order" description:""`
	CreatedAt *time.Time `json:"createdAt" orm:"created_at" description:""`
}
