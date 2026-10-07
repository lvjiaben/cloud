// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentItem is the golang structure for table content_item.
type ContentItem struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int64      `json:"tenantId"        orm:"tenant_id"        description:""`
	ContentNo       string     `json:"contentNo"       orm:"content_no"       description:""`
	ContentType     string     `json:"contentType"     orm:"content_type"     description:""`
	CategoryNo      string     `json:"categoryNo"      orm:"category_no"      description:""`
	VisibilityCode  string     `json:"visibilityCode"  orm:"visibility_code"  description:""`
	Title           string     `json:"title"           orm:"title"            description:""`
	Summary         string     `json:"summary"         orm:"summary"          description:""`
	Body            string     `json:"body"            orm:"body"             description:""`
	SeoTitle        string     `json:"seoTitle"        orm:"seo_title"        description:""`
	SeoDescription  string     `json:"seoDescription"  orm:"seo_description"  description:""`
	PublicPath      string     `json:"publicPath"      orm:"public_path"      description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"      description:""`
	ValidityVersion int64      `json:"validityVersion" orm:"validity_version" description:""`
	CurrentFileNo   string     `json:"currentFileNo"   orm:"current_file_no"  description:""`
	PublishedAt     *time.Time `json:"publishedAt"     orm:"published_at"     description:""`
	OfflineAt       *time.Time `json:"offlineAt"       orm:"offline_at"       description:""`
	DeletedAt       *time.Time `json:"deletedAt"       orm:"deleted_at"       description:""`
	Version         int64      `json:"version"         orm:"version"          description:""`
	CreatedBy       int64      `json:"createdBy"       orm:"created_by"       description:""`
	UpdatedBy       int64      `json:"updatedBy"       orm:"updated_by"       description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"       description:""`
}
