// This file centralizes stable caller-visible provider-configuration error codes.
package providerconfig

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_PROVIDER_INVALID",
		"The compliance provider configuration request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_PROVIDER_NOT_ACCESSIBLE",
		"The compliance provider configuration is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeRecentAuthenticationRequired = bizerr.MustDefine(
		"IDC_COMPLIANCE_PROVIDER_RECENT_AUTH_REQUIRED",
		"A recent full sign-in is required before changing or validating the compliance provider credential",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_PROVIDER_CONFLICT",
		"The compliance provider configuration conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeCredentialUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_PROVIDER_CREDENTIAL_UNAVAILABLE",
		"The compliance provider credential is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_PROVIDER_UNAVAILABLE",
		"The compliance provider configuration is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
