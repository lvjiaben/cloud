// This file defines stable enterprise-verification business errors.

package enterpriseverification

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_ENTERPRISE_VERIFICATION_INVALID",
		"The enterprise verification request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeMaterialUnsupported = bizerr.MustDefine(
		"IDC_COMPLIANCE_ENTERPRISE_MATERIAL_UNSUPPORTED",
		"The enterprise verification material is not supported",
		gcode.CodeInvalidParameter,
	)
	CodeMaterialTooLarge = bizerr.MustDefine(
		"IDC_COMPLIANCE_ENTERPRISE_MATERIAL_TOO_LARGE",
		"The enterprise verification material is too large",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_ENTERPRISE_VERIFICATION_NOT_ACCESSIBLE",
		"The enterprise verification case is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_ENTERPRISE_VERIFICATION_CONFLICT",
		"The enterprise verification request conflicts with current state",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_ENTERPRISE_VERIFICATION_UNAVAILABLE",
		"The enterprise verification service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
