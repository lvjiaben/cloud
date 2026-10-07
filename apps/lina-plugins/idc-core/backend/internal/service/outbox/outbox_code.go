// This file defines stable transactional-outbox business error codes.

package outbox

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports malformed event, worker, lease, or transition input.
	CodeInvalid = bizerr.MustDefine(
		"IDC_OUTBOX_INVALID",
		"The outbox request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeTransactionRequired reports enqueue calls made outside the authoritative transaction.
	CodeTransactionRequired = bizerr.MustDefine(
		"IDC_OUTBOX_TRANSACTION_REQUIRED",
		"The outbox event must be created inside an active database transaction",
		gcode.CodeInvalidOperation,
	)
	// CodeIdempotencyConflict reports changed immutable content under one deduplication scope.
	CodeIdempotencyConflict = bizerr.MustDefine(
		"IDC_OUTBOX_IDEMPOTENCY_CONFLICT",
		"The deduplication key was already used for a different outbox event",
		gcode.CodeOperationFailed,
	)
	// CodeEvidenceInvalid reports malformed or non-redacted event evidence.
	CodeEvidenceInvalid = bizerr.MustDefine(
		"IDC_OUTBOX_EVIDENCE_INVALID",
		"The outbox payload or metadata is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeEvidenceTooLarge reports payload or metadata outside configured storage bounds.
	CodeEvidenceTooLarge = bizerr.MustDefine(
		"IDC_OUTBOX_EVIDENCE_TOO_LARGE",
		"The outbox payload or metadata exceeds the configured size limit",
		gcode.CodeInvalidParameter,
	)
	// CodeLeaseLost intentionally collapses absent, stale, foreign, and expired leases.
	CodeLeaseLost = bizerr.MustDefine(
		"IDC_OUTBOX_LEASE_LOST",
		"The outbox event lease is no longer valid",
		gcode.CodeInvalidOperation,
	)
	// CodeStateConflict reports an unsupported or conflicting event transition.
	CodeStateConflict = bizerr.MustDefine(
		"IDC_OUTBOX_STATE_CONFLICT",
		"The outbox event cannot perform this state transition",
		gcode.CodeInvalidOperation,
	)
	// CodeUnavailable hides lower-level persistence failures while retaining their causes.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_OUTBOX_UNAVAILABLE",
		"The outbox service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
