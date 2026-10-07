// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerAttachment is the golang structure of table plugin_idc_core_customer_attachment for DAO operations like Where/Data.
type CustomerAttachment struct {
	g.Meta           `orm:"table:plugin_idc_core_customer_attachment, do:true"`
	Id               any        //
	TenantId         any        //
	AttachmentNo     any        //
	CustomerNo       any        //
	CategoryCode     any        //
	VisibilityCode   any        //
	ObjectKey        any        // Internal private object key, never a public or long-lived download URL
	OriginalFilename any        //
	MediaType        any        //
	SizeBytes        any        //
	ContentSha256    any        //
	ScanStatusCode   any        //
	StatusCode       any        //
	RetentionUntil   *time.Time //
	UploadedBy       any        //
	AvailableAt      *time.Time //
	RemovedAt        *time.Time //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
