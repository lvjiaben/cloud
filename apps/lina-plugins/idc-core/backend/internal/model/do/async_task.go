// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AsyncTask is the golang structure of table plugin_idc_core_async_task for DAO operations like Where/Data.
type AsyncTask struct {
	g.Meta               `orm:"table:plugin_idc_core_async_task, do:true"`
	Id                   any        //
	TenantId             any        //
	TaskNo               any        // Opaque task business number
	TaskType             any        //
	SubjectType          any        //
	SubjectNo            any        //
	CustomerNo           any        // Validated customer ownership projection for database-side permission filtering
	ExternalReferenceNo  any        // Current normalized external task or operation reference
	IdempotencyKeyDigest any        // SHA-256 digest of the task idempotency key
	StatusCode           any        //
	Priority             any        //
	Payload              any        // Versioned redacted task input without secrets
	ResultSnapshot       any        // Redacted normalized result used for status and recovery
	AvailableAt          *time.Time //
	DeadlineAt           *time.Time //
	LockedBy             any        //
	LeaseExpiresAt       *time.Time //
	AttemptCount         any        //
	MaxAttempts          any        //
	CompletedAt          *time.Time //
	ManualTakeoverAt     *time.Time // Time when automated execution yielded to an operator
	ManualAssigneeUserId any        // Host staff identity assigned to manual takeover
	ManualReason         any        // Redacted operator-facing reason for manual takeover
	LastErrorCode        any        //
	LastErrorSummary     any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
