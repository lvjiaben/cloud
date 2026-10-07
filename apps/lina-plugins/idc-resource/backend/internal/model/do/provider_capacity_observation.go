// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProviderCapacityObservation is the golang structure of table plugin_idc_resource_provider_capacity_observation for DAO operations like Where/Data.
type ProviderCapacityObservation struct {
	g.Meta                 `orm:"table:plugin_idc_resource_provider_capacity_observation, do:true"`
	Id                     any        //
	TenantId               any        //
	ObservationNo          any        //
	ChannelNo              any        //
	CapacityKind           any        //
	StatusCode             any        //
	ObservationFingerprint any        //
	ItemCount              any        //
	ObservedAt             *time.Time //
	CompletedAt            *time.Time //
	CreatedAt              *time.Time //
	PageSize               any        // Fixed Provider page size for the exact collecting observation
	NextCursor             any        // Exact opaque Provider cursor required for the next staged page
	PageCount              any        // Number of validated pages durably staged into this observation
	TriggerCode            any        // Trusted operation source: system or direct staff manual request
	RequestedBy            any        // Trusted host staff user identifier for a manual request; zero for system work
	AttemptCount           any        // Bounded number of claimed Provider page attempts for this observation
	NextAttemptAt          *time.Time // PostgreSQL due time for the next single-page attempt; null means not scheduled
	LeaseTokenDigest       any        // SHA-256 digest of an ephemeral worker lease token; the token itself is never stored
	LeaseExpiresAt         *time.Time // PostgreSQL time after which another worker may reclaim this collecting observation
	FailureCode            any        // Stable non-secret terminal failure code; raw Provider errors are forbidden
}
