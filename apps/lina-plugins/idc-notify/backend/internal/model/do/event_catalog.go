// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EventCatalog is the golang structure of table plugin_idc_notify_event_catalog for DAO operations like Where/Data.
type EventCatalog struct {
	g.Meta            `orm:"table:plugin_idc_notify_event_catalog, do:true"`
	TenantId          any        //
	EventCode         any        //
	TopicCode         any        //
	ObjectType        any        //
	RequiredVariables any        //
	OptionalVariables any        //
	SecurityRequired  any        //
	Enabled           any        //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
