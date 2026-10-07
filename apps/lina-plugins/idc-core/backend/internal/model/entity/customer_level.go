// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerLevel is the golang structure for table customer_level.
type CustomerLevel struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	LevelNo         string     `json:"levelNo"         orm:"level_no"          description:""`
	LevelCode       string     `json:"levelCode"       orm:"level_code"        description:""`
	NameZhCn        string     `json:"nameZhCn"        orm:"name_zh_cn"        description:""`
	NameEnUs        string     `json:"nameEnUs"        orm:"name_en_us"        description:""`
	DescriptionZhCn string     `json:"descriptionZhCn" orm:"description_zh_cn" description:""`
	DescriptionEnUs string     `json:"descriptionEnUs" orm:"description_en_us" description:""`
	Priority        int        `json:"priority"        orm:"priority"          description:""`
	IsDefault       bool       `json:"isDefault"       orm:"is_default"        description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	Version         int64      `json:"version"         orm:"version"           description:""`
	CreatedBy       int        `json:"createdBy"       orm:"created_by"        description:""`
	UpdatedBy       int        `json:"updatedBy"       orm:"updated_by"        description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
