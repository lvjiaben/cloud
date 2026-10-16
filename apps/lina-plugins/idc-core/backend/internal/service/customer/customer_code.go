// This file defines stable customer-profile business errors.

package customer

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeVerificationRequired identifies an otherwise available customer whose
	// identity has not passed the verification required by this operation.
	CodeVerificationRequired = bizerr.MustDefine(
		"IDC_CUSTOMER_VERIFICATION_REQUIRED",
		"Customer identity verification is required before this operation",
		gcode.CodeInvalidOperation,
	)
	// CodeProfileInvalid reports malformed self-service profile input.
	CodeProfileInvalid = bizerr.MustDefine(
		"IDC_CUSTOMER_PROFILE_INVALID",
		"The customer profile request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodePhoneExists = bizerr.MustDefine(
		"IDC_CUSTOMER_PHONE_EXISTS",
		"The phone number already belongs to a customer",
		gcode.CodeInvalidOperation,
	)
	CodeEmailExists = bizerr.MustDefine(
		"IDC_CUSTOMER_EMAIL_EXISTS",
		"The email address already belongs to a customer",
		gcode.CodeInvalidOperation,
	)
	// CodeNotAccessible deliberately collapses unbound, missing, malformed, and
	// foreign customer subjects so a caller cannot enumerate customer numbers.
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_CUSTOMER_NOT_ACCESSIBLE",
		"The customer profile is not accessible",
		gcode.CodeNotAuthorized,
	)
	// CodeStateDenied reports a known owned customer whose current lifecycle
	// state does not allow the requested self-service mutation.
	CodeStateDenied = bizerr.MustDefine(
		"IDC_CUSTOMER_STATE_DENIED",
		"The customer account state does not allow this operation",
		gcode.CodeInvalidOperation,
	)
	// CodeInitialPasswordChangeRequired requires a directly created customer to
	// replace the administrator-provided password before using account APIs.
	CodeInitialPasswordChangeRequired = bizerr.MustDefine(
		"IDC_CUSTOMER_INITIAL_PASSWORD_CHANGE_REQUIRED",
		"The initial customer password must be changed before continuing",
		gcode.CodeNotAuthorized,
	)
	// CodeVersionConflict reports an optimistic concurrency mismatch.
	CodeVersionConflict = bizerr.MustDefine(
		"IDC_CUSTOMER_VERSION_CONFLICT",
		"The customer profile changed before this operation completed",
		gcode.CodeInvalidOperation,
	)
	// CodeIdempotencyConflict reports reuse of one key for different semantics.
	CodeIdempotencyConflict = bizerr.MustDefine(
		"IDC_CUSTOMER_IDEMPOTENCY_CONFLICT",
		"The idempotency key was already used for a different customer operation",
		gcode.CodeInvalidOperation,
	)
	// CodeWorkflowInvalid reports malformed or unsupported account workflow input.
	CodeWorkflowInvalid = bizerr.MustDefine(
		"IDC_CUSTOMER_WORKFLOW_INVALID",
		"The customer account workflow request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeWorkflowUnavailable reports an unavailable or malformed required
	// dependency projection. The operation fails closed and persists no request.
	CodeWorkflowUnavailable = bizerr.MustDefine(
		"IDC_CUSTOMER_WORKFLOW_UNAVAILABLE",
		"The customer account workflow is temporarily unavailable",
		gcode.CodeInternalError,
	)
	// CodeUnavailable hides persistence and integrity details while retaining
	// their causes for server-side diagnostics.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_CUSTOMER_UNAVAILABLE",
		"The customer profile service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
