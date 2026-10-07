// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentDownloadAuthorization is the golang structure for table content_download_authorization.
type ContentDownloadAuthorization struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	TenantId  int64      `json:"tenantId"  orm:"tenant_id"  description:""`
	ContentNo string     `json:"contentNo" orm:"content_no" description:""`
	UserId    int64      `json:"userId"    orm:"user_id"    description:""`
	Revision  int64      `json:"revision"  orm:"revision"   description:""`
	UpdatedAt *time.Time `json:"updatedAt" orm:"updated_at" description:""`
}
