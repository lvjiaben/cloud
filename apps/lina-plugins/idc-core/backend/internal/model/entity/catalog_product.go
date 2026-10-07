// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProduct is the golang structure for table catalog_product.
type CatalogProduct struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	ProductNo        string     `json:"productNo"        orm:"product_no"         description:""`
	CategoryNo       string     `json:"categoryNo"       orm:"category_no"        description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"        description:""`
	PublicSlug       string     `json:"publicSlug"       orm:"public_slug"        description:""`
	NameZhCn         string     `json:"nameZhCn"         orm:"name_zh_cn"         description:""`
	NameEnUs         string     `json:"nameEnUs"         orm:"name_en_us"         description:""`
	SummaryZhCn      string     `json:"summaryZhCn"      orm:"summary_zh_cn"      description:""`
	SummaryEnUs      string     `json:"summaryEnUs"      orm:"summary_en_us"      description:""`
	CurrentVersionNo *string    `json:"currentVersionNo" orm:"current_version_no" description:""`
	SortOrder        int        `json:"sortOrder"        orm:"sort_order"         description:""`
	Version          int64      `json:"version"          orm:"version"            description:""`
	CreatedBy        int        `json:"createdBy"        orm:"created_by"         description:""`
	UpdatedBy        int        `json:"updatedBy"        orm:"updated_by"         description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"         description:""`
	DeletedAt        *time.Time `json:"deletedAt"        orm:"deleted_at"         description:""`
}
