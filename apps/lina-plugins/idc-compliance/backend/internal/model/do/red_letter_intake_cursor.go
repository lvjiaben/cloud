// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterIntakeCursor is the golang structure of table plugin_idc_compliance_red_letter_intake_cursor for DAO operations like Where/Data.
type RedLetterIntakeCursor struct {
	g.Meta       `orm:"table:plugin_idc_compliance_red_letter_intake_cursor, do:true"`
	Id           any        //
	TenantId     any        //
	FeedCode     any        //
	SourceCursor any        //
	Version      any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
