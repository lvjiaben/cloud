// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductDirectory is the golang structure for table product_directory.
type ProductDirectory struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	DirectoryNo    string     `json:"directoryNo"    orm:"directory_no"    description:""`
	NameZhCn       string     `json:"nameZhCn"       orm:"name_zh_cn"      description:""`
	NameEnUs       string     `json:"nameEnUs"       orm:"name_en_us"      description:""`
	SortOrder      int        `json:"sortOrder"      orm:"sort_order"      description:""`
	StatusCode     string     `json:"statusCode"     orm:"status_code"     description:""`
	Version        int64      `json:"version"        orm:"version"         description:""`
	CreatedBy      int        `json:"createdBy"      orm:"created_by"      description:""`
	UpdatedBy      int        `json:"updatedBy"      orm:"updated_by"      description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"      description:""`
	NavigationType string     `json:"navigationType" orm:"navigation_type" description:""`
	TargetUrl      string     `json:"targetUrl"      orm:"target_url"      description:""`
	Icon           string     `json:"icon"           orm:"icon"            description:""`
}
