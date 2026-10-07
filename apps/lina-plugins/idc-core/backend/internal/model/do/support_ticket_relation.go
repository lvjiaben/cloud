// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketRelation is the golang structure of table plugin_idc_core_support_ticket_relation for DAO operations like Where/Data.
type SupportTicketRelation struct {
	g.Meta         `orm:"table:plugin_idc_core_support_ticket_relation, do:true"`
	Id             any        //
	TenantId       any        //
	RelationNo     any        //
	RelationType   any        //
	SourceTicketNo any        //
	TargetTicketNo any        //
	ActorUserId    any        //
	Reason         any        //
	CreatedAt      *time.Time //
}
