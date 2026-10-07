// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Task is the golang structure of table plugin_idc_resource_task for DAO operations like Where/Data.
type Task struct {
	g.Meta                  `orm:"table:plugin_idc_resource_task, do:true"`
	Id                      any        //
	TenantId                any        //
	TaskNo                  any        //
	RequestIdDigest         any        // SHA-256 digest of the caller idempotency key; raw keys are not persisted
	CapabilityCode          any        //
	LocalResourceNo         any        //
	ChannelNo               any        //
	PoolNo                  any        //
	StatusCode              any        //
	Priority                any        //
	SanitizedParameters     any        // Validated non-secret operation parameters; credential handles and plaintext are forbidden
	AttemptCount            any        //
	MaxAttempts             any        //
	AvailableAt             *time.Time //
	LockedBy                any        //
	LeaseExpiresAt          *time.Time //
	NextObserveAt           *time.Time //
	AcceptedAt              *time.Time //
	FinishedAt              *time.Time //
	FailureCode             any        //
	FailureSummary          any        //
	ResultUnknown           any        // Hard replay fence requiring observation or manual reconciliation
	Version                 any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
	ObservationClaimedAt    *time.Time // PostgreSQL time of the latest QueryResource observation claim; never derived from updated_at
	OwnerMetadata           any        // Validated non-secret owner intent retained for replay checks and never sent to the Provider
	ObservationAttemptCount any        // Consecutive read-only reconciliation attempts; never authorizes Provider write replay
}
