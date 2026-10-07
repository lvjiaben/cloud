// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentQualificationEvent is the golang structure of table plugin_idc_core_sales_agent_qualification_event for DAO operations like Where/Data.
type SalesAgentQualificationEvent struct {
	g.Meta          `orm:"table:plugin_idc_core_sales_agent_qualification_event, do:true"`
	Id              any        //
	TenantId        any        //
	EventNo         any        //
	QualificationNo any        //
	EventSequence   any        //
	FromStatusCode  any        //
	ToStatusCode    any        //
	FromLevelNo     any        //
	ToLevelNo       any        //
	ReasonCode      any        //
	ActorReference  any        //
	EvidenceDigest  any        //
	OccurredAt      *time.Time //
}
