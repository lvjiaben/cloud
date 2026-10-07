// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// HealthSnapshot is the golang structure of table plugin_idc_resource_health_snapshot for DAO operations like Where/Data.
type HealthSnapshot struct {
	g.Meta                     `orm:"table:plugin_idc_resource_health_snapshot, do:true"`
	Id                         any        //
	TenantId                   any        //
	SnapshotNo                 any        //
	ChannelNo                  any        //
	HealthCode                 any        //
	LatencyMs                  any        //
	ConsecutiveFailures        any        //
	CircuitOpen                any        //
	CapabilityObservedAt       *time.Time //
	ProviderCapacityObservedAt *time.Time //
	SummaryCode                any        // Stable non-sensitive machine code; raw upstream errors are forbidden
	ObservedAt                 *time.Time //
	CreatedAt                  *time.Time //
	OperationKind              any        //
}
