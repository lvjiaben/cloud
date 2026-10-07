// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyIncidentEvent is the golang structure of table plugin_idc_compliance_privacy_incident_event for DAO operations like Where/Data.
type PrivacyIncidentEvent struct {
	g.Meta      `orm:"table:plugin_idc_compliance_privacy_incident_event, do:true"`
	Id          any        //
	TenantId    any        //
	EventNo     any        //
	IncidentNo  any        //
	StageCode   any        //
	SafeNote    any        //
	ActorUserId any        //
	OccurredAt  *time.Time //
	CreatedAt   *time.Time //
}
