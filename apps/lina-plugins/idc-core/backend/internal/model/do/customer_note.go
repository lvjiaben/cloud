// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerNote is the golang structure of table plugin_idc_core_customer_note for DAO operations like Where/Data.
type CustomerNote struct {
	g.Meta           `orm:"table:plugin_idc_core_customer_note, do:true"`
	Id               any        //
	TenantId         any        //
	NoteNo           any        //
	CustomerNo       any        //
	CategoryCode     any        //
	Content          any        //
	StatusCode       any        //
	SupersedesNoteNo any        //
	AuthorUserId     any        //
	WithdrawnBy      any        //
	WithdrawnAt      *time.Time //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
