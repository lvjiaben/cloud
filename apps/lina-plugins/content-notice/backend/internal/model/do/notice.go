// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Notice is the golang structure of table plugin_content_notice for DAO operations like Where/Data.
type Notice struct {
	g.Meta    `orm:"table:plugin_content_notice, do:true"`
	Id        any        // Notice ID
	TenantId  any        // Owning tenant ID, 0 means PLATFORM
	Title     any        // Notice title
	Content   any        // Notice content
	Status    any        // Notice status: 0=draft, 1=published
	CreatedBy any        // Creator
	UpdatedBy any        // Updater
	CreatedAt *time.Time // Creation time
	UpdatedAt *time.Time // Update time
	DeletedAt *time.Time // Deletion time
	SortOrder any        //
}
