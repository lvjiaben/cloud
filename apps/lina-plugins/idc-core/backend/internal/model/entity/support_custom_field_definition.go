// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportCustomFieldDefinition is the golang structure for table support_custom_field_definition.
type SupportCustomFieldDefinition struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int64      `json:"tenantId"        orm:"tenant_id"         description:""`
	FieldNo         string     `json:"fieldNo"         orm:"field_no"          description:""`
	FieldType       string     `json:"fieldType"       orm:"field_type"        description:""`
	NameZhCn        string     `json:"nameZhCn"        orm:"name_zh_cn"        description:""`
	NameEnUs        string     `json:"nameEnUs"        orm:"name_en_us"        description:""`
	DescriptionZhCn string     `json:"descriptionZhCn" orm:"description_zh_cn" description:""`
	DescriptionEnUs string     `json:"descriptionEnUs" orm:"description_en_us" description:""`
	AppliesPresales bool       `json:"appliesPresales" orm:"applies_presales"  description:""`
	AppliesCustomer bool       `json:"appliesCustomer" orm:"applies_customer"  description:""`
	AppliesInternal bool       `json:"appliesInternal" orm:"applies_internal"  description:""`
	VisibilityCode  string     `json:"visibilityCode"  orm:"visibility_code"   description:""`
	Required        bool       `json:"required"        orm:"required"          description:""`
	SortOrder       int        `json:"sortOrder"       orm:"sort_order"        description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	Version         int64      `json:"version"         orm:"version"           description:""`
	CreatedBy       int64      `json:"createdBy"       orm:"created_by"        description:""`
	UpdatedBy       int64      `json:"updatedBy"       orm:"updated_by"        description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
