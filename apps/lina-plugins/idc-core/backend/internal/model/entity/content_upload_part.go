// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentUploadPart is the golang structure for table content_upload_part.
type ContentUploadPart struct {
	Id          int64      `json:"id"          orm:"id"           description:""`
	TenantId    int64      `json:"tenantId"    orm:"tenant_id"    description:""`
	UploadNo    string     `json:"uploadNo"    orm:"upload_no"    description:""`
	PartNumber  int        `json:"partNumber"  orm:"part_number"  description:""`
	OffsetBytes int64      `json:"offsetBytes" orm:"offset_bytes" description:""`
	SizeBytes   int        `json:"sizeBytes"   orm:"size_bytes"   description:""`
	Etag        string     `json:"etag"        orm:"etag"         description:""`
	ObjectRef   string     `json:"objectRef"   orm:"object_ref"   description:""`
	Sha256Hex   string     `json:"sha256Hex"   orm:"sha256_hex"   description:""`
	CreatedAt   *time.Time `json:"createdAt"   orm:"created_at"   description:""`
}
