// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NoticeRead is the golang structure of table plugin_content_notice_read for DAO operations like Where/Data.
type NoticeRead struct {
	g.Meta    `orm:"table:plugin_content_notice_read, do:true"`
	Id        any        //
	NoticeId  any        // Announcement ID
	UserId    any        // Reader user ID
	CreatedAt *time.Time // Read time
}
