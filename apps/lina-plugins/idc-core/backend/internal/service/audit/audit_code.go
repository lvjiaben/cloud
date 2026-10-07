// This file defines stable business errors for the audit service boundary.

package audit

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports malformed audit semantics, metadata, filters, or evidence.
	CodeInvalid = bizerr.MustDefine(
		"IDC_AUDIT_INVALID",
		"The audit request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeIdentityDenied intentionally collapses missing, inconsistent, or unsupported actors.
	CodeIdentityDenied = bizerr.MustDefine(
		"IDC_AUDIT_IDENTITY_DENIED",
		"The current identity cannot perform this audit operation",
		gcode.CodeNotAuthorized,
	)
	// CodePermissionDenied reports missing exact management authority or unsupported data scope.
	CodePermissionDenied = bizerr.MustDefine(
		"IDC_AUDIT_PERMISSION_DENIED",
		"Audit access is not permitted",
		gcode.CodeNotAuthorized,
	)
	// CodeTransactionRequired reports transition writes outside the authoritative transaction.
	CodeTransactionRequired = bizerr.MustDefine(
		"IDC_AUDIT_TRANSACTION_REQUIRED",
		"Audit transitions require an active business transaction",
		gcode.CodeInvalidOperation,
	)
	// CodeIdempotencyConflict reports changed immutable semantics under one request scope.
	CodeIdempotencyConflict = bizerr.MustDefine(
		"IDC_AUDIT_IDEMPOTENCY_CONFLICT",
		"The audit idempotency key was already used for different evidence",
		gcode.CodeInvalidOperation,
	)
	// CodeEvidenceTooLarge reports evidence outside the configured finite bounds.
	CodeEvidenceTooLarge = bizerr.MustDefine(
		"IDC_AUDIT_EVIDENCE_TOO_LARGE",
		"The audit evidence exceeds its configured safety bound",
		gcode.CodeInvalidParameter,
	)
	// CodeUnavailable hides persistence and corrupted-evidence details while retaining causes.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_AUDIT_UNAVAILABLE",
		"The audit service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
