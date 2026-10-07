// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentUploadPart is the golang structure of table plugin_idc_core_content_upload_part for DAO operations like Where/Data.
type ContentUploadPart struct {
	g.Meta      `orm:"table:plugin_idc_core_content_upload_part, do:true"`
	Id          any        //
	TenantId    any        //
	UploadNo    any        //
	PartNumber  any        //
	OffsetBytes any        //
	SizeBytes   any        //
	Etag        any        //
	ObjectRef   any        //
	Sha256Hex   any        //
	CreatedAt   *time.Time //
}
