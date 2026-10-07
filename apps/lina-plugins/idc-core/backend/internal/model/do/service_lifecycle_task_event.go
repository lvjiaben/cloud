// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecycleTaskEvent is the golang structure of table plugin_idc_core_service_lifecycle_task_event for DAO operations like Where/Data.
type ServiceLifecycleTaskEvent struct {
	g.Meta         `orm:"table:plugin_idc_core_service_lifecycle_task_event, do:true"`
	Id             any        //
	TenantId       any        //
	EventNo        any        //
	TaskNo         any        //
	ServiceNo      any        //
	EventSequence  any        //
	FromStatusCode *string    //
	ToStatusCode   any        //
	ReasonCode     any        //
	ActorTypeCode  any        //
	ActorReference any        //
	EvidenceDigest any        //
	ContentDigest  any        //
	CreatedAt      *time.Time //
}
