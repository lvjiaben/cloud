// This file defines stable template-management business errors.

package electroniccontracttemplate

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_INVALID",
		"The electronic contract template request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_NOT_ACCESSIBLE",
		"The electronic contract template is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_CONFLICT",
		"The electronic contract template conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeProviderUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_PROVIDER_UNAVAILABLE",
		"Tencent electronic contract templates are temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeSubmissionUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_SUBMISSION_UNAVAILABLE",
		"The verified Tencent contract does not currently support creating a template from this local draft",
		gcode.CodeNotSupported,
	)
	CodeContentInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_CONTENT_INVALID",
		"The contract template body or field placement is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeConversionUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_CONVERSION_UNAVAILABLE",
		"The installation-local PDF converter is unavailable or could not create a safe preview",
		gcode.CodeNotSupported,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_TEMPLATE_UNAVAILABLE",
		"The electronic contract template service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
