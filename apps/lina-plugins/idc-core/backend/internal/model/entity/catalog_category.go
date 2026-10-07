// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogCategory is the golang structure for table catalog_category.
type CatalogCategory struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	CategoryNo       string     `json:"categoryNo"       orm:"category_no"        description:""`
	ParentCategoryNo *string    `json:"parentCategoryNo" orm:"parent_category_no" description:""`
	NameZhCn         string     `json:"nameZhCn"         orm:"name_zh_cn"         description:""`
	NameEnUs         string     `json:"nameEnUs"         orm:"name_en_us"         description:""`
	SortOrder        int        `json:"sortOrder"        orm:"sort_order"         description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"        description:""`
	Version          int64      `json:"version"          orm:"version"            description:""`
	CreatedBy        int        `json:"createdBy"        orm:"created_by"         description:""`
	UpdatedBy        int        `json:"updatedBy"        orm:"updated_by"         description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"         description:""`
	DeletedAt        *time.Time `json:"deletedAt"        orm:"deleted_at"         description:""`
}
