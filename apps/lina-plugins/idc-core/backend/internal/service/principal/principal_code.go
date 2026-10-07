// This file defines stable principal-audience and onboarding business error codes.

package principal

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeIdentityRequired reports that no authenticated host identity is available.
	CodeIdentityRequired = bizerr.MustDefine(
		"IDC_IDENTITY_REQUIRED",
		"An authenticated identity is required",
		gcode.CodeNotAuthorized,
	)
	// CodeDefaultTenantRequired reports that IDC business traffic escaped tenant 0.
	CodeDefaultTenantRequired = bizerr.MustDefine(
		"IDC_DEFAULT_TENANT_REQUIRED",
		"IDC business operations require the default merchant context",
		gcode.CodeNotAuthorized,
	)
	// CodeAdminAudienceRequired reports that staff binding lacks effective IDC authority.
	CodeAdminAudienceRequired = bizerr.MustDefine(
		"IDC_ADMIN_AUDIENCE_REQUIRED",
		"IDC management permission is required",
		gcode.CodeNotAuthorized,
	)
	// CodeAudienceConflict reports an attempted implicit customer/staff conversion.
	CodeAudienceConflict = bizerr.MustDefine(
		"IDC_AUDIENCE_CONFLICT",
		"The identity is already bound to an incompatible audience",
		gcode.CodeInvalidOperation,
	)
	// CodeAudienceInconsistent reports a persisted audience and customer binding mismatch.
	CodeAudienceInconsistent = bizerr.MustDefine(
		"IDC_AUDIENCE_INCONSISTENT",
		"The identity audience is inconsistent",
		gcode.CodeInvalidOperation,
	)
	// CodePrincipalUnavailable hides lower-level identity persistence failures
	// from callers while retaining their cause for server-side diagnostics.
	CodePrincipalUnavailable = bizerr.MustDefine(
		"IDC_PRINCIPAL_UNAVAILABLE",
		"IDC identity service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	// CodeOnboardingUnavailable intentionally collapses invalid, expired, replayed-by-other,
	// or otherwise unusable onboarding intents into one non-enumerating response.
	CodeOnboardingUnavailable = bizerr.MustDefine(
		"IDC_ONBOARDING_UNAVAILABLE",
		"Customer onboarding cannot be completed",
		gcode.CodeInvalidOperation,
	)
)
