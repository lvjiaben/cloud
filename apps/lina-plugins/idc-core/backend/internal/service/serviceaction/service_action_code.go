package serviceaction

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_SERVICE_ACTION_INVALID",
		"The service operation request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_SERVICE_ACTION_NOT_ACCESSIBLE",
		"The requested service is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_SERVICE_ACTION_CONFLICT",
		"The service operation conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeNotReady = bizerr.MustDefine(
		"IDC_SERVICE_ACTION_NOT_READY",
		"The requested service content is not ready yet",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_SERVICE_ACTION_UNAVAILABLE",
		"The service operation is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
