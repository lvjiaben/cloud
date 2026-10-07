// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentItem is the golang structure of table plugin_idc_core_content_item for DAO operations like Where/Data.
type ContentItem struct {
	g.Meta          `orm:"table:plugin_idc_core_content_item, do:true"`
	Id              any        //
	TenantId        any        //
	ContentNo       any        //
	ContentType     any        //
	CategoryNo      any        //
	VisibilityCode  any        //
	Title           any        //
	Summary         any        //
	Body            any        //
	SeoTitle        any        //
	SeoDescription  any        //
	PublicPath      any        //
	StatusCode      any        //
	ValidityVersion any        //
	CurrentFileNo   any        //
	PublishedAt     *time.Time //
	OfflineAt       *time.Time //
	DeletedAt       *time.Time //
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
