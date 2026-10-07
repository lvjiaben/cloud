// This file defines stable asynchronous-task business error codes.

package asynctask

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports malformed task, lease, or transition input.
	CodeInvalid = bizerr.MustDefine(
		"IDC_ASYNC_TASK_INVALID",
		"The asynchronous task request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeIdempotencyConflict reports changed task content under one idempotency key.
	CodeIdempotencyConflict = bizerr.MustDefine(
		"IDC_ASYNC_TASK_IDEMPOTENCY_CONFLICT",
		"The idempotency key was already used for a different task request",
		gcode.CodeInvalidOperation,
	)
	// CodeTransactionRequired reports task creation outside its owning business transaction.
	CodeTransactionRequired = bizerr.MustDefine(
		"IDC_ASYNC_TASK_TRANSACTION_REQUIRED",
		"Asynchronous tasks must be created inside the owning business transaction",
		gcode.CodeInvalidOperation,
	)
	// CodeLeaseLost intentionally collapses absent, stale, foreign, and expired leases.
	CodeLeaseLost = bizerr.MustDefine(
		"IDC_ASYNC_TASK_LEASE_LOST",
		"The asynchronous task lease is no longer valid",
		gcode.CodeInvalidOperation,
	)
	// CodeStateConflict reports an unsupported state transition such as unsafe takeover.
	CodeStateConflict = bizerr.MustDefine(
		"IDC_ASYNC_TASK_STATE_CONFLICT",
		"The asynchronous task cannot perform this state transition",
		gcode.CodeInvalidOperation,
	)
	// CodeUnavailable hides lower-level persistence failures while retaining their causes.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_ASYNC_TASK_UNAVAILABLE",
		"The asynchronous task service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
