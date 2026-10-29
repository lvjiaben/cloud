// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// InterfaceCandidateSnapshot is the golang structure of table plugin_idc_resource_interface_candidate_snapshot for DAO operations like Where/Data.
type InterfaceCandidateSnapshot struct {
	g.Meta                `orm:"table:plugin_idc_resource_interface_candidate_snapshot, do:true"`
	RequestKey            any        //
	TenantId              any        //
	InterfaceInstanceNo   any        //
	ConfigurationRevision any        //
	RequestPayload        any        //
	ResultPayload         any        //
	ObservedAt            *time.Time //
	RefreshAfter          *time.Time //
	LeaseUntil            *time.Time //
	LastErrorCode         any        //
}
