// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// OutboxEvent is the golang structure of table plugin_idc_core_outbox_event for DAO operations like Where/Data.
type OutboxEvent struct {
	g.Meta              `orm:"table:plugin_idc_core_outbox_event, do:true"`
	Id                  any        //
	TenantId            any        //
	EventNo             any        // Opaque outbox event business number
	EventType           any        //
	EventVersion        any        // Positive version of the published event contract
	Topic               any        // Stable logical delivery topic
	AggregateType       any        //
	AggregateNo         any        //
	CorrelationNo       any        // Opaque business correlation number
	DeduplicationDigest any        // Digest that prevents duplicate events of the same type
	StatusCode          any        //
	Payload             any        // Versioned redacted event payload without secrets
	Metadata            any        // Versioned delivery metadata without secrets
	OccurredAt          *time.Time //
	AvailableAt         *time.Time // Earliest time at which the event may be claimed
	LockedBy            any        //
	LeaseExpiresAt      *time.Time // Claim lease expiry used for crash recovery
	AttemptCount        any        //
	MaxAttempts         any        //
	PublishedAt         *time.Time //
	LastErrorCode       any        //
	LastErrorSummary    any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
