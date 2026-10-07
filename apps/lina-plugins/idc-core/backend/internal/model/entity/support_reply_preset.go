// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportReplyPreset is the golang structure for table support_reply_preset.
type SupportReplyPreset struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int64      `json:"tenantId"     orm:"tenant_id"     description:""`
	PresetNo     string     `json:"presetNo"     orm:"preset_no"     description:""`
	DepartmentNo string     `json:"departmentNo" orm:"department_no" description:""`
	Locale       string     `json:"locale"       orm:"locale"        description:""`
	Title        string     `json:"title"        orm:"title"         description:""`
	Body         string     `json:"body"         orm:"body"          description:""`
	SortOrder    int        `json:"sortOrder"    orm:"sort_order"    description:""`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedBy    int64      `json:"createdBy"    orm:"created_by"    description:""`
	UpdatedBy    int64      `json:"updatedBy"    orm:"updated_by"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
