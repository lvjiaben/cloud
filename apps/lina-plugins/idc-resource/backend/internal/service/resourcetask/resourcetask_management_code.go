// This file defines stable business errors for Provider task administration.

package resourcetask

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeTaskManagementInvalid identifies malformed list, detail, or takeover input.
	CodeTaskManagementInvalid = bizerr.MustDefine(
		"IDC_RESOURCE_TASK_MANAGEMENT_INVALID",
		"The Provider task request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeTaskManagementNotAccessible hides unauthorized, missing, or inaccessible tasks.
	CodeTaskManagementNotAccessible = bizerr.MustDefine(
		"IDC_RESOURCE_TASK_MANAGEMENT_NOT_ACCESSIBLE",
		"The Provider task is not accessible",
		gcode.CodeNotAuthorized,
	)
	// CodeTaskManagementConflict identifies a stale or no-longer-eligible takeover.
	CodeTaskManagementConflict = bizerr.MustDefine(
		"IDC_RESOURCE_TASK_MANAGEMENT_CONFLICT",
		"The Provider task conflicts with the current state",
		gcode.CodeInvalidOperation,
	)
	// CodeTaskManagementUnavailable hides database and audit infrastructure details.
	CodeTaskManagementUnavailable = bizerr.MustDefine(
		"IDC_RESOURCE_TASK_MANAGEMENT_UNAVAILABLE",
		"The Provider task service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
