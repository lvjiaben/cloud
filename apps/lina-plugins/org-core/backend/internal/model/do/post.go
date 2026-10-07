// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Post is the golang structure of table plugin_org_core_post for DAO operations like Where/Data.
type Post struct {
	g.Meta    `orm:"table:plugin_org_core_post, do:true"`
	Id        any        // Post ID
	TenantId  any        // Owning tenant ID, 0 means PLATFORM
	DeptId    any        // Owning department ID
	Code      any        // Post code
	Name      any        // Post name
	Sort      any        // Display order
	Status    any        // Status: 0=disabled, 1=enabled
	Remark    any        // Remark
	CreatedAt *time.Time // Creation time
	UpdatedAt *time.Time // Update time
	DeletedAt *time.Time // Deletion time
}
