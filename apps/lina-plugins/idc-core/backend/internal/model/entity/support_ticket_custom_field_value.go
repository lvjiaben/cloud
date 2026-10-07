// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketCustomFieldValue is the golang structure for table support_ticket_custom_field_value.
type SupportTicketCustomFieldValue struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int64      `json:"tenantId"            orm:"tenant_id"             description:""`
	TicketNo            string     `json:"ticketNo"            orm:"ticket_no"             description:""`
	FieldNo             string     `json:"fieldNo"             orm:"field_no"              description:""`
	DefinitionVersion   int64      `json:"definitionVersion"   orm:"definition_version"    description:""`
	FieldType           string     `json:"fieldType"           orm:"field_type"            description:""`
	NameZhCn            string     `json:"nameZhCn"            orm:"name_zh_cn"            description:""`
	NameEnUs            string     `json:"nameEnUs"            orm:"name_en_us"            description:""`
	DescriptionZhCn     string     `json:"descriptionZhCn"     orm:"description_zh_cn"     description:""`
	DescriptionEnUs     string     `json:"descriptionEnUs"     orm:"description_en_us"     description:""`
	VisibilityCode      string     `json:"visibilityCode"      orm:"visibility_code"       description:""`
	Required            bool       `json:"required"            orm:"required"              description:""`
	SortOrder           int        `json:"sortOrder"           orm:"sort_order"            description:""`
	NormalizedValue     string     `json:"normalizedValue"     orm:"normalized_value"      description:""`
	SelectedOptionCount int        `json:"selectedOptionCount" orm:"selected_option_count" description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
