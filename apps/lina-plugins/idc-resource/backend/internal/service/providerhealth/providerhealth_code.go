// This file defines the localized business error codes for Provider health management.

package providerhealth

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeHealthManagementInvalid identifies invalid health list or policy input.
	CodeHealthManagementInvalid = bizerr.MustDefine(
		"IDC_RESOURCE_HEALTH_MANAGEMENT_INVALID",
		"The Provider health request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeHealthManagementNotAccessible hides unauthorized or inaccessible health targets.
	CodeHealthManagementNotAccessible = bizerr.MustDefine(
		"IDC_RESOURCE_HEALTH_MANAGEMENT_NOT_ACCESSIBLE",
		"The Provider health target is not accessible",
		gcode.CodeNotAuthorized,
	)
	// CodeHealthManagementConflict identifies stale or unsafe policy updates.
	CodeHealthManagementConflict = bizerr.MustDefine(
		"IDC_RESOURCE_HEALTH_MANAGEMENT_CONFLICT",
		"The Provider health policy conflicts with the current state",
		gcode.CodeInvalidOperation,
	)
	// CodeHealthManagementUnavailable hides storage and audit infrastructure details.
	CodeHealthManagementUnavailable = bizerr.MustDefine(
		"IDC_RESOURCE_HEALTH_MANAGEMENT_UNAVAILABLE",
		"The Provider health service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
