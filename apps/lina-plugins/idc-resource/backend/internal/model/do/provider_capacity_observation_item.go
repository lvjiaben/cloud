// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProviderCapacityObservationItem is the golang structure of table plugin_idc_resource_provider_capacity_observation_item for DAO operations like Where/Data.
type ProviderCapacityObservationItem struct {
	g.Meta        `orm:"table:plugin_idc_resource_provider_capacity_observation_item, do:true"`
	Id            any        //
	TenantId      any        //
	ObservationNo any        //
	CapacityKind  any        //
	ExternalRef   any        //
	Projection    any        // Normalized non-secret projection; raw Provider responses are forbidden
	ObservedAt    *time.Time //
	CreatedAt     *time.Time //
}
