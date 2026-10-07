// This file defines stable secret-service business error codes.

package secret

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports malformed scope, handle, idempotency, version, or plaintext input.
	CodeInvalid = bizerr.MustDefine(
		"IDC_SECRET_INVALID",
		"The secret request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeAccessDenied deliberately hides authorization failure details and secret existence.
	CodeAccessDenied = bizerr.MustDefine(
		"IDC_SECRET_ACCESS_DENIED",
		"Secret access is not permitted",
		gcode.CodeNotAuthorized,
	)
	// CodeNotConfigured collapses absent, revoked, and foreign-scope handles.
	CodeNotConfigured = bizerr.MustDefine(
		"IDC_SECRET_NOT_CONFIGURED",
		"The secret is not configured",
		gcode.CodeNotFound,
	)
	// CodeIdempotencyConflict rejects changed plaintext under one idempotency key.
	CodeIdempotencyConflict = bizerr.MustDefine(
		"IDC_SECRET_IDEMPOTENCY_CONFLICT",
		"The idempotency key is already linked to different secret content",
		gcode.CodeOperationFailed,
	)
	// CodeVersionConflict reports stale optimistic update or revoke state.
	CodeVersionConflict = bizerr.MustDefine(
		"IDC_SECRET_VERSION_CONFLICT",
		"The secret changed before this operation completed",
		gcode.CodeInvalidOperation,
	)
	// CodeConsumerFailed hides callback errors that could contain copied plaintext.
	CodeConsumerFailed = bizerr.MustDefine(
		"IDC_SECRET_CONSUMER_FAILED",
		"The authorized secret operation failed",
		gcode.CodeOperationFailed,
	)
	// CodeUnavailable hides provider, cryptographic, and persistence failure details.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_SECRET_UNAVAILABLE",
		"The secret service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
