// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentDownloadAuthorization is the golang structure of table plugin_idc_core_content_download_authorization for DAO operations like Where/Data.
type ContentDownloadAuthorization struct {
	g.Meta    `orm:"table:plugin_idc_core_content_download_authorization, do:true"`
	Id        any        //
	TenantId  any        //
	ContentNo any        //
	UserId    any        //
	Revision  any        //
	UpdatedAt *time.Time //
}
