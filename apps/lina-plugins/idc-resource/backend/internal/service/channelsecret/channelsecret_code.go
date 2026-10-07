package channelsecret

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_RESOURCE_CHANNEL_SECRET_INVALID",
		"The channel secret request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_RESOURCE_CHANNEL_SECRET_NOT_ACCESSIBLE",
		"The channel secret target is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeRecentAuthenticationRequired = bizerr.MustDefine(
		"IDC_RESOURCE_CHANNEL_SECRET_RECENT_AUTHENTICATION_REQUIRED",
		"Sign in again before changing channel secrets",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_RESOURCE_CHANNEL_SECRET_CONFLICT",
		"The channel secret changed; refresh and try again",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_RESOURCE_CHANNEL_SECRET_UNAVAILABLE",
		"The channel secret service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
