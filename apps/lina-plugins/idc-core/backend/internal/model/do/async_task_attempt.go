// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AsyncTaskAttempt is the golang structure of table plugin_idc_core_async_task_attempt for DAO operations like Where/Data.
type AsyncTaskAttempt struct {
	g.Meta              `orm:"table:plugin_idc_core_async_task_attempt, do:true"`
	Id                  any        //
	TenantId            any        //
	TaskNo              any        //
	AttemptNo           any        // Monotonic attempt number within one task
	WorkerId            any        //
	ExternalReferenceNo any        // External reference returned for this execution attempt
	StatusCode          any        //
	RequestSnapshot     any        // Redacted normalized request evidence
	ResponseSnapshot    any        // Redacted normalized response evidence
	StartedAt           *time.Time //
	EndedAt             *time.Time //
	ErrorCode           any        //
	ErrorSummary        any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
